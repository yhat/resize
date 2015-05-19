package resize

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/ec2"
)

func TestInstanceTypes(t *testing.T) {
	_, err := InstanceTypes(nil)
	if err != nil {
		t.Fatal(err)
	}
}

// taken from http://cloud-images.ubuntu.com/locator/ec2/
var UbuntuInstances = map[string]string{
	"ap-northeast-1": "ami-d4c807d4",
	"ap-southeast-1": "ami-84f0cfd6",
	"ap-southeast-2": "ami-af027d95",
	"cn-north-1":     "ami-12c8552b",
	"eu-central-1":   "ami-48c5fa55",
	"eu-west-1":      "ami-b97a12ce",
	"sa-east-1":      "ami-65991e78",
	"us-east-1":      "ami-76b2a71e",
	"us-gov-west-1":  "ami-0b365628",
	"us-west-1":      "ami-af7f90eb",
	"us-west-2":      "ami-3789b807",
}

// TestInstanceResize starts a micro instance and attempts to resize it as
// something larger
func TestInstanceResize(t *testing.T) {
	accessKey, secretKey := awsCreds(t)
	regionName := os.Getenv("AWS_TEST_REGION")
	if regionName == "" {
		t.Skip("AWS_TEST_REGION environment variable not set, skipping test")
	}
	region, ok := aws.Regions[regionName]
	if !ok {
		t.Skip("unknown aws region " + regionName)
	}

	amiId, ok := UbuntuInstances[regionName]
	if !ok {
		t.Skip("no ubuntu image for region " + regionName)

	}
	ec2Cli := ec2.New(aws.Auth{
		AccessKey: accessKey,
		SecretKey: secretKey,
	}, region)

	ops := ec2.RunInstances{
		ImageId:      amiId,
		MinCount:     1,
		MaxCount:     1,
		InstanceType: "t2.small",
	}
	resp, err := ec2Cli.RunInstances(&ops)
	if err != nil {
		t.Fatal(err)
	}
	defer func(instances []ec2.Instance) {
		ids := make([]string, len(instances))
		for i := range instances {
			ids[i] = instances[i].InstanceId
		}
		_, err := ec2Cli.TerminateInstances(ids)
		if err != nil {
			t.Error(err)
		}
	}(resp.Instances)
	if len(resp.Instances) != 1 {
		t.Errorf("bad number of instances started: %d", len(resp.Instances))
		return
	}
	instance := resp.Instances[0]

	//Make sure the test instance is in the running state before we proceed
	var pollUntilRunning = func(ec2Cli *ec2.EC2, id string) error {
		for i := 0; i < 10; i++ {
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
					code = status.InstanceState.Code
				}
			}
			if code == -1 {
				return fmt.Errorf("State not available for test instance")
			} else if code == 0 {
				continue
			} else if code == 16 {
				return nil
			}
		}
		return fmt.Errorf("instance did not reach running state")
	}
	if err := pollUntilRunning(ec2Cli, instance.InstanceId); err != nil {
		t.Error(err)
		return
	}
	if err := stopAndWait(ec2Cli, instance.InstanceId); err != nil {
		t.Error(err)
		return
	}
	if err := resize(ec2Cli, instance.InstanceId, "t2.medium"); err != nil {
		t.Error(err)
		return
	}
	//The size change won't be reflected until the instance is up and running
	if err := pollUntilRunning(ec2Cli, instance.InstanceId); err != nil {
		t.Error(err)
		return
	}
	time.Sleep(time.Second * 3)
	instanceResp, err := ec2Cli.Instances([]string{instance.InstanceId}, nil)
	if err != nil {
		t.Error(err)
		return
	}
	for _, r := range instanceResp.Reservations {
		for _, i := range r.Instances {
			if i.InstanceId == instance.InstanceId {
				if i.InstanceType != "t2.medium" {
					t.Errorf("expected instance type to be t2.medium, but it was %s",
						i.InstanceType)
				}
				return
			}
		}
	}

	t.Error("Test instance not found")
}
