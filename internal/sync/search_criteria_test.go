package sync

import "testing"

func TestBuildSearchCriteriaCoversAddressSubjectAndBody(t *testing.T) {
	const query = "quarterly report"
	criteria := buildSearchCriteria(query)
	if criteria == nil || len(criteria.Or) != 1 {
		t.Fatalf("top-level criteria = %#v", criteria)
	}

	from := criteria.Or[0][0]
	if len(from.Header) != 1 || from.Header[0].Key != "FROM" || from.Header[0].Value != query {
		t.Fatalf("FROM criteria = %#v", from)
	}
	subjectPair := criteria.Or[0][1].Or
	if len(subjectPair) != 1 || len(subjectPair[0][0].Header) != 1 || subjectPair[0][0].Header[0].Key != "SUBJECT" || subjectPair[0][0].Header[0].Value != query {
		t.Fatalf("SUBJECT criteria = %#v", subjectPair)
	}
	toPair := subjectPair[0][1].Or
	if len(toPair) != 1 || len(toPair[0][0].Header) != 1 || toPair[0][0].Header[0].Key != "TO" || toPair[0][0].Header[0].Value != query {
		t.Fatalf("TO criteria = %#v", toPair)
	}
	ccPair := toPair[0][1].Or
	if len(ccPair) != 1 || len(ccPair[0][0].Header) != 1 || ccPair[0][0].Header[0].Key != "CC" || ccPair[0][0].Header[0].Value != query {
		t.Fatalf("CC criteria = %#v", ccPair)
	}
	if len(ccPair[0][1].Body) != 1 || ccPair[0][1].Body[0] != query {
		t.Fatalf("BODY criteria = %#v", ccPair[0][1])
	}
}
