package resize

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/goamz/ec2"
	"github.com/yhat/scrape"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const instanceTypeURL = "http://aws.amazon.com/ec2/instance-types/"

type InstanceType struct {
	Name               string  // col 0
	CPUs               int     // col 1
	Memory             float64 // GiB col 2
	Storage            string  // GB col 3
	NetworkSpec        string  // col 4
	Processor          string  // col 5
	ClockSpeed         float64 // GHz col 6
	IntelAVX           bool    // col 7
	IntelAVX2          bool    // col 8
	IntelTurbo         bool    // col 9
	EBSOPT             bool    // col 10
	EnhancedNetworking bool    // col 11
}

// parseRow parses a row from the instance types matrix into it's given
// InstanceType
func parseRow(row *html.Node) (InstanceType, error) {
	cols := scrape.Find(row, scrape.ByTag(atom.Td))
	if len(cols) != 12 {
		return InstanceType{}, fmt.Errorf("expected 12 columns, got %d", len(cols))
	}
	yesNo := func(col *html.Node) bool {
		return strings.ToLower(scrape.Text(col)) == "yes"
	}
	t := InstanceType{
		Name:               scrape.Text(cols[0]),
		Storage:            scrape.Text(cols[3]),
		NetworkSpec:        scrape.Text(cols[4]),
		Processor:          scrape.Text(cols[5]),
		IntelAVX:           yesNo(cols[7]),
		IntelAVX2:          yesNo(cols[8]),
		IntelTurbo:         yesNo(cols[9]),
		EBSOPT:             yesNo(cols[10]),
		EnhancedNetworking: yesNo(cols[11]),
	}
	var err error
	t.CPUs, err = strconv.Atoi(scrape.Text(cols[1]))
	if err != nil {
		err = fmt.Errorf("expected number for CPUs, got '%s'", scrape.Text(cols[1]))
		return InstanceType{}, err
	}
	t.Memory, err = strconv.ParseFloat(scrape.Text(cols[2]), 64)
	if err != nil {
		err = fmt.Errorf("expected number for Memory, got '%s'", scrape.Text(cols[2]))
		return InstanceType{}, err
	}

	t.ClockSpeed, err = strconv.ParseFloat(scrape.Text(cols[6]), 64)
	if err != nil {
		err = fmt.Errorf("expected number for Memory, got '%s'", scrape.Text(cols[2]))
		return InstanceType{}, err
	}
	return t, nil
}

// InstanceTypes makes a request to AWS and parses the current available EC2
// instance types. Since this information is not available from the EC2 api,
// we must scrape it ourselves.
func InstanceTypes(client *http.Client) ([]InstanceType, error) {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Get(instanceTypeURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad response from AWS: %s", resp.Status)
	}

	root, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	var findMatrix func(node *html.Node) (*html.Node, bool)
	findMatrix = func(node *html.Node) (*html.Node, bool) {
		if scrape.Attr(node, "id") == "instance-type-matrix" {
			return node, true
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			n, ok := findMatrix(c)
			if ok {
				return n, true
			}
		}
		return nil, false
	}
	matrixHeader, ok := findMatrix(root)
	if !ok {
		return nil, fmt.Errorf("no node with id 'instance-type-matrix'")
	}

	contains := func(sli []string, ele string) bool {
		for _, s := range sli {
			if s == ele {
				return true
			}
		}
		return false
	}

	var section *html.Node
	for section = matrixHeader.Parent; section != nil; section = section.Parent {
		classes := strings.Fields(scrape.Attr(section, "class"))
		if contains(classes, "section") && contains(classes, "title-wrapper") {
			break
		}
	}
	if section == nil {
		return nil, fmt.Errorf("malformed HTML: title-wrapper not found")
	}
	var next *html.Node
	for next = section.NextSibling; next != nil; next = next.NextSibling {
		classes := strings.Fields(scrape.Attr(next, "class"))
		if contains(classes, "section") && contains(classes, "table-wrapper") {
			break
		}
	}
	if next == nil {
		return nil, fmt.Errorf("malformed HTML: table-wrapper not found")
	}
	rows := scrape.Find(next, scrape.ByTag(atom.Tr))

	if len(rows) < 3 {
		return nil, fmt.Errorf("malformed HTML: could not find table")
	}
	rows = rows[1:]
	types := make([]InstanceType, len(rows))
	for i, row := range rows {
		types[i], err = parseRow(row)
		if err != nil {
			return nil, err
		}
	}
	return types, nil
}

func openIps(ec2Cli *ec2.EC2) (open []ec2.Address, err error) {
	resp, err := ec2Cli.Addresses(nil, nil, nil)
	for _, addr := range resp.Addresses {
		if addr.AssociationId == "" {
			open = append(open, addr)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("error getting addresses: %v", err)
	}
	return open, nil
}

func stopAndWait(ec2Cli *ec2.EC2, w io.Writer, id string) error {
	if _, err := ec2Cli.StopInstances(id); err != nil {
		return fmt.Errorf("error stopping instance: %v", err)
	}
	for i := 0; i < 20; i++ {
		time.Sleep(time.Second * 3)
		opts := ec2.DescribeInstanceStatus{
			InstanceIds:         []string{id},
			IncludeAllInstances: true,
		}
		resp, err := ec2Cli.DescribeInstanceStatus(&opts, nil)
		if err != nil {
			return fmt.Errorf("error checking instance status: %v", err)
		}
		code := -1
		for _, status := range resp.InstanceStatus {
			if status.InstanceId == id {
				e := Event{Status: "message", Message: status.InstanceState.Name}
				b, err := json.Marshal(e)
				if err != nil {
					return fmt.Errorf("error marshalling JSON: %v", err)
				}
				w.Write(b)
				code = status.InstanceState.Code
			}
		}
		if code == -1 {
			return fmt.Errorf("instance status not available")
		} else if code == 0 || code == 64 {
			continue
		} else if code == 80 {
			return nil
		} else {
			return fmt.Errorf("unexpected instance state: %s", code)
		}
	}
	return fmt.Errorf("timed out waiting for instance to reach 'stopped' state")
}

func pollUntilRunning(ec2Cli *ec2.EC2, w io.Writer, id string) error {
	for i := 0; i < 20; i++ {
		time.Sleep(time.Second * 2)
		opts := ec2.DescribeInstanceStatus{
			InstanceIds:         []string{id},
			IncludeAllInstances: true,
		}
		resp, err := ec2Cli.DescribeInstanceStatus(&opts, nil)
		if err != nil {
			return fmt.Errorf("error getting instance status: %v", err)
		}
		code := -1
		for _, status := range resp.InstanceStatus {
			if status.InstanceId == id {
				e := Event{Status: "message", Message: status.InstanceState.Name}
				b, err := json.Marshal(e)
				if err != nil {
					return fmt.Errorf("error marshalling JSON: %v", err)
				}
				w.Write(b)
				code = status.InstanceState.Code
			}
		}
		if code == -1 {
			return fmt.Errorf("Could not get state for this instance")
		} else if code == 0 {
			continue
		} else if code == 16 {
			return nil
		}
	}
	return fmt.Errorf("Timed out waiting for instance to reach running state")
}

func resize(ec2Cli *ec2.EC2, id string, newType string) error {
	ops := ec2.ModifyInstance{InstanceType: newType}
	resp, err := ec2Cli.ModifyInstance(id, &ops)
	if err != nil {
		return fmt.Errorf("error modifying instance: %v", err)
	}
	if !resp.Return {
		return fmt.Errorf("bad response from AWS")
	}
	return nil
}

func allocateIp(ec2Cli *ec2.EC2, instanceId string, allocId string) error {
	opts := &ec2.AssociateAddress{
		InstanceId:         instanceId,
		AllocationId:       allocId,
		AllowReassociation: false,
	}
	resp, err := ec2Cli.AssociateAddress(opts)
	if err != nil {
		return fmt.Errorf("could not associate address: %v", err)
	}
	if !resp.Return {
		return fmt.Errorf("bad response from AWS")
	}
	return nil
}
