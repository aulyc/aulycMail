package contact

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDisplayNameAndOwnAddressNormalizationEdges(t *testing.T) {
	for _, test := range []struct {
		input string
		want  string
	}{
		{"", ""},
		{" plain name ", "plain name"},
		{`"quoted name"`, "quoted name"},
		{`"escaped \"quote\" and \\ slash"`, `escaped "quote" and \ slash`},
		{`"trailing\\"`, `trailing\`},
	} {
		if got := unquoteDisplayName(test.input); got != test.want {
			t.Fatalf("unquoteDisplayName(%q) = %q, want %q", test.input, got, test.want)
		}
	}

	store := NewStore(openTestDB(t).DB)
	store.SetOwnEmails([]string{" Owner@Example.com ", "", "ALIAS@example.com", "owner@example.com"})
	if !store.IsOwnEmail("owner@example.com") || !store.IsOwnEmail(" ALIAS@EXAMPLE.COM ") {
		t.Fatal("normalized own addresses were not recognized")
	}
	if store.IsOwnEmail("") || store.IsOwnEmail("other@example.com") {
		t.Fatal("empty or unrelated address was treated as owned")
	}
	if got := mapSourceForLegacy("remote"); got != "remote" {
		t.Fatalf("remote legacy source = %q", got)
	}
}

func TestRichRecordRoundTripPreservesMetadataAndReplacesSubtables(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db.DB)
	now := time.Now().UTC().Truncate(time.Second)
	record := &Record{
		ID:             "rich-record",
		Source:         "local",
		Kind:           "manual",
		Fn:             "Rich Person",
		NGiven:         "Rich",
		NFamily:        "Person",
		Org:            "Example Org",
		Title:          "Engineer",
		Note:           "synthetic note",
		Bday:           "2000-01-02",
		Nickname:       "RP",
		PhotoData:      "c3ludGhldGlj",
		PhotoMediaType: "image/png",
		Emails: []RecordEmail{
			{Email: " PRIMARY@Example.com ", EmailType: "work", SendCount: 2, LastUsed: now},
			{Email: "secondary@example.com", EmailType: "home", IsPrimary: true, NameOverridden: true},
			{Email: "   "},
		},
		Phones: []RecordPhone{
			{Number: " 12345 ", PhoneType: "mobile"},
			{Number: "67890", PhoneType: "work", IsPrimary: true},
			{Number: "   "},
		},
		Addresses: []RecordAddress{
			{AddrType: "work", Street: "1 Test Road", City: "Beijing", Region: "Beijing", Postcode: "100000", Country: "CN"},
			{},
		},
		URLs: []RecordURL{
			{URL: " https://example.com ", URLType: "homepage"},
			{},
		},
		IMPPs: []RecordIMPP{
			{Handle: " im:rich ", IMPPType: "xmpp"},
			{},
		},
		Categories: []string{" Friends ", "", "Friends", "Work"},
	}
	if err := store.UpsertRecord(record); err != nil {
		t.Fatalf("initial rich UpsertRecord: %v", err)
	}
	if record.CreatedAt.IsZero() || record.UpdatedAt.IsZero() {
		t.Fatalf("upsert timestamps = created:%v updated:%v", record.CreatedAt, record.UpdatedAt)
	}

	got, err := store.GetRecord("rich-record")
	if err != nil || got == nil {
		t.Fatalf("GetRecord rich result = %#v, %v", got, err)
	}
	if got.Fn != "Rich Person" || got.NGiven != "Rich" || got.NFamily != "Person" || got.Org != "Example Org" || got.PhotoMediaType != "image/png" {
		t.Fatalf("rich scalar fields = %#v", got)
	}
	if len(got.Emails) != 2 || len(got.Phones) != 2 || len(got.Addresses) != 1 || len(got.URLs) != 1 || len(got.IMPPs) != 1 || len(got.Categories) != 2 {
		t.Fatalf("rich subtable sizes = emails:%d phones:%d addresses:%d urls:%d impps:%d categories:%d", len(got.Emails), len(got.Phones), len(got.Addresses), len(got.URLs), len(got.IMPPs), len(got.Categories))
	}
	if !got.Emails[0].IsPrimary || got.Emails[0].Email != "primary@example.com" || got.Emails[0].LastUsed.IsZero() {
		t.Fatalf("primary email normalization/metadata = %#v", got.Emails[0])
	}

	if _, err := db.Exec(`UPDATE contact_emails SET send_count = 9, name_overridden = 1 WHERE record_id = ? AND email = ?`, record.ID, "primary@example.com"); err != nil {
		t.Fatal(err)
	}
	replacement := &Record{
		ID:        record.ID,
		Source:    "local",
		Kind:      "manual",
		Fn:        "Replaced Person",
		CreatedAt: record.CreatedAt,
		Emails:    []RecordEmail{{Email: "primary@example.com"}},
		Phones:    []RecordPhone{{Number: "99999"}},
		PhotoURL:  "https://example.com/avatar.png",
	}
	if err := store.UpsertRecord(replacement); err != nil {
		t.Fatalf("replacement UpsertRecord: %v", err)
	}
	got, err = store.GetRecordByEmail(" PRIMARY@EXAMPLE.COM ")
	if err != nil || got == nil {
		t.Fatalf("GetRecordByEmail replacement = %#v, %v", got, err)
	}
	if got.Fn != "Replaced Person" || got.PhotoURL == "" || len(got.Emails) != 1 || got.Emails[0].SendCount != 9 || !got.Emails[0].NameOverridden || len(got.Phones) != 1 || len(got.Addresses) != 0 {
		t.Fatalf("replacement record = %#v", got)
	}

	listed, err := store.ListRecords(RecordFilter{Source: "local", Kind: "manual", Query: "primary", Limit: 1, Offset: 0})
	if err != nil || len(listed) != 1 || listed[0].ID != record.ID {
		t.Fatalf("filtered rich records = %#v, %v", listed, err)
	}
	summaries, total, err := store.ListRecordSummaries(RecordFilter{Source: "local", SortOrder: RecordSortNameDesc, Limit: 5, Offset: -2})
	if err != nil || len(summaries) != 1 || total != 1 || summaries[0].PrimaryEmail != "primary@example.com" {
		t.Fatalf("rich summaries = %#v total=%d err=%v", summaries, total, err)
	}
}

func TestContactAPINeutralInputsAndClosedDatabaseFailures(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db.DB)

	if err := store.UpdateName("", "ignored"); err == nil {
		t.Fatal("UpdateName accepted an empty email")
	}
	if err := store.Create("", "ignored"); err == nil {
		t.Fatal("Create accepted an empty email")
	}
	if got, err := store.Get(""); got != nil || err != nil {
		t.Fatalf("Get(empty) = %#v, %v", got, err)
	}
	if got, err := store.GetRecordByEmail(""); got != nil || err != nil {
		t.Fatalf("GetRecordByEmail(empty) = %#v, %v", got, err)
	}
	if got, err := store.GetRecord(""); got != nil || err != nil {
		t.Fatalf("GetRecord(empty) = %#v, %v", got, err)
	}
	if err := store.Delete(""); err != nil {
		t.Fatalf("Delete(empty) = %v", err)
	}
	if err := store.PurgeOwnEmail(""); err != nil {
		t.Fatalf("PurgeOwnEmail(empty) = %v", err)
	}
	if err := store.DeleteRecord(""); err != nil {
		t.Fatalf("DeleteRecord(empty) = %v", err)
	}
	if err := store.UpdateRecordName("", "ignored"); err != nil {
		t.Fatalf("UpdateRecordName(empty) = %v", err)
	}
	if got, err := store.ListAssociatedAccountsForEmails([]string{"", "  "}); got != nil || err != nil {
		t.Fatalf("ListAssociatedAccountsForEmails(empty) = %#v, %v", got, err)
	}
	if got, err := store.Search("anything", 0); err != nil || len(got) != 0 {
		t.Fatalf("Search default limit result = %#v, %v", got, err)
	}

	for _, invalid := range []*Record{
		nil,
		{ID: "id"},
		{ID: "id", Source: "remote"},
		{Source: "local"},
	} {
		if err := UpsertRecordTx(db.DB, invalid); err == nil {
			t.Fatalf("UpsertRecordTx accepted invalid record %#v", invalid)
		}
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	assertError := func(name string, err error) {
		t.Helper()
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "closed") {
			t.Fatalf("%s error = %v, want closed database error", name, err)
		}
	}

	results, err := store.Search("closed", 2)
	if err != nil || results == nil || len(results) != 0 {
		t.Fatalf("Search intentionally degrades closed DB failure = %#v, %v", results, err)
	}
	assertError("UpdateName", store.UpdateName("x@example.com", "X"))
	assertError("Create", store.Create("x@example.com", "X"))
	_, err = store.Get("x@example.com")
	assertError("Get", err)
	assertError("Delete", store.Delete("x@example.com"))
	_, err = store.List(0)
	assertError("List", err)
	_, err = store.Count()
	assertError("Count", err)
	_, err = store.GetRecordByEmail("x@example.com")
	assertError("GetRecordByEmail", err)
	_, err = store.GetRecord("record")
	assertError("GetRecord", err)
	_, err = store.ListRecords(RecordFilter{})
	assertError("ListRecords", err)
	_, _, err = store.ListRecordSummaries(RecordFilter{})
	assertError("ListRecordSummaries", err)
	_, err = store.CountRecords(RecordFilter{})
	assertError("CountRecords", err)
	_, err = store.ListAccountAssociations()
	assertError("ListAccountAssociations", err)
	_, err = store.ListAssociatedAccountsForEmails([]string{"x@example.com"})
	assertError("ListAssociatedAccountsForEmails", err)
	assertError("UpsertRecord", store.UpsertRecord(&Record{ID: "record", Source: "local"}))
	assertError("DeleteRecord", store.DeleteRecord("record"))
	assertError("UpdateRecordName", store.UpdateRecordName("record", "Name"))

	if !errors.Is(ErrContactExists, ErrContactExists) {
		t.Fatal("contact sentinel lost errors.Is identity")
	}
}
