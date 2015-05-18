package resize

import (
	"os"
	"testing"

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
	err = resize(ec2Cli, instance, "t2.medium")
	if err != nil {
		t.Error(err)
		return
	}
}
