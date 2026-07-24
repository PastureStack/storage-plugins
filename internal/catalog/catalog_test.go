package catalog

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/PastureStack/storage-plugins/internal/model"
)

func TestCapabilitiesGolden(t *testing.T) {
	actual, err := json.Marshal(Output("en-US"))
	if err != nil {
		t.Fatal(err)
	}
	actual = append(actual, '\n')
	expected, err := os.ReadFile("testdata/capabilities.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatalf("capability output changed\nactual: %s\nexpected: %s", actual, expected)
	}
}

func TestSevenDriversAreSortedAndControlsAreDisabled(t *testing.T) {
	output := Output("en-US")
	if len(output.Drivers) != 7 {
		t.Fatalf("got %d drivers, want 7", len(output.Drivers))
	}
	for index := 1; index < len(output.Drivers); index++ {
		if output.Drivers[index-1].Driver >= output.Drivers[index].Driver {
			t.Fatalf("drivers are not strictly sorted: %q before %q", output.Drivers[index-1].Driver, output.Drivers[index].Driver)
		}
	}
	if !reflect.DeepEqual(output.Controls, model.Controls{}) {
		t.Fatalf("controls must all be disabled: %#v", output.Controls)
	}
	if len(output.DelegatedComponents) != 2 {
		t.Fatalf("got %d delegated components, want 2", len(output.DelegatedComponents))
	}
	for _, component := range output.DelegatedComponents {
		if component.Included {
			t.Fatalf("delegated component must not be included: %#v", component)
		}
	}
}
