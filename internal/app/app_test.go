package app

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/PastureStack/storage-plugins/internal/contract"
	"github.com/PastureStack/storage-plugins/internal/model"
)

const validInput = `{"apiVersion":"storage-plugins.pasturestack.io/v1alpha1","kind":"LifecyclePlanRequest","driver":"aws-ebs","operation":"create","idempotencyKey":"request-ebs","ownerId":"team-a","expectedGeneration":0,"observedState":{"phase":"absent","generation":0,"ownerId":""},"desired":{"phase":"available","allowIrreversible":true,"config":{"volumeType":"gp3","sizeGiB":100,"iops":3000,"encrypted":true,"kmsRef":"ref:kms-policy-a"}}}`

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) {
	panic("capabilities must not read stdin")
}

func TestCapabilitiesDoesNotReadStdin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"capabilities", "--locale", "en-US"}, panicReader{}, &stdout, &stderr)
	if code != exitSuccess {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	var output model.CapabilitiesOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Drivers) != 7 || len(output.DelegatedComponents) != 2 {
		t.Fatalf("unexpected capability output: %#v", output)
	}
	if !reflect.DeepEqual(output.Controls, model.Controls{}) {
		t.Fatalf("controls enabled: %#v", output.Controls)
	}
}

func TestValidateAndPlanUseOnlyStdinJSON(t *testing.T) {
	for _, command := range []string{"validate", "plan"} {
		t.Run(command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run([]string{command, "--locale", "zh-TW"}, strings.NewReader(validInput), &stdout, &stderr)
			if code != exitSuccess {
				t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
			}
			if !json.Valid(stdout.Bytes()) {
				t.Fatalf("stdout is not JSON: %s", stdout.String())
			}
			if strings.Contains(stdout.String(), "kms-policy-a") {
				t.Fatal("output echoed an external reference")
			}
			if !strings.Contains(stdout.String(), `"execution":false`) {
				t.Fatalf("output does not expose disabled execution: %s", stdout.String())
			}
		})
	}
}

func TestSchemaErrorsAreGenericAndDoNotEchoRejectedValues(t *testing.T) {
	input := strings.Replace(validInput, `"encrypted":true`, `"encrypted":true,"token":"do-not-echo-this"`, 1)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"plan", "--locale", "en-US"}, strings.NewReader(input), &stdout, &stderr)
	if code != exitInvalid {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("invalid structural request wrote stdout: %s", stdout.String())
	}
	if strings.Contains(stderr.String(), "do-not-echo-this") || strings.Contains(stderr.String(), "token") {
		t.Fatalf("error echoed rejected input: %s", stderr.String())
	}
	var output model.ErrorOutput
	if err := json.Unmarshal(stderr.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if output.Code != "invalid-request" {
		t.Fatalf("code=%q", output.Code)
	}
}

func TestValidateAndPlanRejectNullWithGenericError(t *testing.T) {
	input := strings.Replace(validInput, `"kmsRef":"ref:kms-policy-a"`, `"kmsRef":null`, 1)
	for _, command := range []string{"validate", "plan"} {
		t.Run(command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run([]string{command, "--locale", "en-US"}, strings.NewReader(input), &stdout, &stderr)
			if code != exitInvalid {
				t.Fatalf("%s exit=%d, want %d", command, code, exitInvalid)
			}
			if stdout.Len() != 0 {
				t.Fatalf("%s wrote stdout for null input: %s", command, stdout.String())
			}
			if strings.Contains(stderr.String(), "kmsRef") || strings.Contains(stderr.String(), "ref:kms-policy-a") || strings.Contains(strings.ToLower(stderr.String()), "null") {
				t.Fatalf("%s error exposed rejected input: %s", command, stderr.String())
			}
			var output model.ErrorOutput
			if err := json.Unmarshal(stderr.Bytes(), &output); err != nil {
				t.Fatal(err)
			}
			if output.Code != contract.ErrorInvalidJSON {
				t.Fatalf("%s code=%q, want %q", command, output.Code, contract.ErrorInvalidJSON)
			}
		})
	}
}

func TestLocaleAndArgumentNamesAreExact(t *testing.T) {
	for _, args := range [][]string{
		{"capabilities", "--locale", "en-us"},
		{"capabilities", "extra"},
		{"unknown", "--locale", "en-US"},
	} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, strings.NewReader(""), &stdout, &stderr); code != exitInvalid {
			t.Fatalf("args=%v exit=%d", args, code)
		}
		if !json.Valid(stderr.Bytes()) {
			t.Fatalf("args=%v stderr is not JSON: %s", args, stderr.String())
		}
	}
}
