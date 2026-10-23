package request

import "testing"

func TestTestResultsNormalizeAndValidate(t *testing.T) {
	request := StartTest{Application: " inventory ", Version: " 1.2.3 "}
	if err := request.NormalizeAndValidate(); err != nil {
		t.Fatal(err)
	}
	if request.Application != "inventory" || request.Version != "1.2.3" {
		t.Fatalf("normalized request = %#v", request)
	}
}

func TestTestResultsRejectInvalidFields(t *testing.T) {
	tests := []StartTest{
		{},
		{Application: "inventory"},
		{Application: string(make([]byte, 129)), Version: "1.0"},
	}
	for _, request := range tests {
		if err := request.NormalizeAndValidate(); err == nil {
			t.Fatalf("expected invalid request to fail: %#v", request)
		}
	}
}

func TestGetTestResultsValidatesPathID(t *testing.T) {
	if err := (GetTestResults{TestID: "run-123"}).Validate(); err != nil {
		t.Fatal(err)
	}
	for _, testID := range []string{"", "   ", string(make([]byte, 129))} {
		if err := (GetTestResults{TestID: testID}).Validate(); err == nil {
			t.Fatalf("expected invalid test ID %q to fail", testID)
		}
	}
}
