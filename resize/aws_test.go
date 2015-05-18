package resize

import "testing"

func TestInstanceTypes(t *testing.T) {
	_, err := InstanceTypes(nil)
	if err != nil {
		t.Fatal(err)
	}
}
