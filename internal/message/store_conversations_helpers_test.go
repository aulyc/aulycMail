package message

import (
	"reflect"
	"testing"
)

func TestParseAggregatedToListJSONFlattensAndDeduplicatesRecipients(t *testing.T) {
	for _, input := range []string{"", "[]", "not-json"} {
		if got := parseAggregatedToListJSON(input); got != nil {
			t.Fatalf("parseAggregatedToListJSON(%q) = %#v, want nil", input, got)
		}
	}

	got := parseAggregatedToListJSON(`[[{"name":"Alice","email":"ALICE@example.com"},{"name":"Empty","email":""}],[{"name":"Duplicate","email":"alice@example.com"},{"name":"Bob","email":"bob@example.com"}]]`)
	want := []Address{
		{Name: "Alice", Email: "ALICE@example.com"},
		{Name: "Bob", Email: "bob@example.com"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseAggregatedToListJSON() = %#v, want %#v", got, want)
	}
}
