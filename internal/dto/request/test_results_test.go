package request

import "testing"

func TestTestResultsNormalizeAndValidate(t *testing.T) {
	request := TestResults{Application: " inventory ", Version: " 1.2.3 "}
	if err := request.NormalizeAndValidate(); err != nil {
		t.Fatal(err)
	}
	if request.Application != "inventory" || request.Version != "1.2.3" {
		t.Fatalf("normalized request = %#v", request)
	}
}

func TestTestResultsRejectInvalidFields(t *testing.T) {
	tests := []TestResults{
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
