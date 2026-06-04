package session

import (
	"context"
	"errors"
	"testing"

	"termua/internal/opcua"
)

func variable(nodeID string) opcua.AddressNode {
	return opcua.AddressNode{NodeID: nodeID, DisplayName: nodeID, NodeClass: "Variable"}
}

func TestSelectVariableRequestsSubscribeAndDetails(t *testing.T) {
	set := NewInspectionSet()
	requests := set.Select(variable("ns=2;s=Level"))

	assertKinds(t, requests, RequestSubscribeValue, RequestReadDetails)
	selected, ok := set.Selected()
	if !ok || !selected.Subscribing || !selected.LoadingDetails {
		t.Fatalf("selected inspection = %#v, ok=%t", selected, ok)
	}
}

func TestWatchReusesSelectedInspection(t *testing.T) {
	set := NewInspectionSet()
	set.Select(variable("ns=2;s=Level"))

	requests := set.Watch(variable("ns=2;s=Level"))
	if len(requests) != 0 {
		t.Fatalf("expected no duplicate requests, got %#v", requests)
	}
	selected, _ := set.Selected()
	if !selected.Watched {
		t.Fatalf("expected selected inspection promoted to Watchlist: %#v", selected)
	}
}

func TestSelectedAndWatchedShareStaleValueState(t *testing.T) {
	set := NewInspectionSet()
	set.Select(variable("ns=2;s=Level"))
	set.Watch(variable("ns=2;s=Level"))
	set.ApplyLiveValue("ns=2;s=Level", opcua.LiveValue{}, errors.New("subscription closed"))

	selected, _ := set.Selected()
	watched := set.Watched()
	if !selected.Stale || len(watched) != 1 || !watched[0].Stale {
		t.Fatalf("selected=%#v watched=%#v", selected, watched)
	}
}

func TestOutOfRangeComputedFromLiveValueAndRange(t *testing.T) {
	set := NewInspectionSet()
	set.Select(variable("ns=2;s=Level"))
	set.ApplyDetails("ns=2;s=Level", opcua.NodeDetails{NodeID: "ns=2;s=Level", EURange: &opcua.ValueRange{Low: 0, High: 100}}, nil)
	set.ApplyLiveValue("ns=2;s=Level", opcua.LiveValue{NodeID: "ns=2;s=Level", Value: "120"}, nil)

	selected, _ := set.Selected()
	if selected.OutOfRange != "120 is above 100" {
		t.Fatalf("OutOfRange = %q", selected.OutOfRange)
	}
}

func TestWatchedInspectionRemainsAliveWhenUnselected(t *testing.T) {
	set := NewInspectionSet()
	set.Select(variable("ns=2;s=Level"))
	set.Watch(variable("ns=2;s=Level"))

	requests := set.Unselect()
	if len(requests) != 0 {
		t.Fatalf("expected no cancellation, got %#v", requests)
	}
	if len(set.Watched()) != 1 {
		t.Fatalf("expected watched inspection to remain alive")
	}
}

func TestUnwatchedUnselectedInspectionRequestsCancellation(t *testing.T) {
	set := NewInspectionSet()
	set.Watch(variable("ns=2;s=Level"))
	sub := &fakeSubscription{}
	set.ApplySubscription("ns=2;s=Level", make(chan opcua.LiveValue), sub, nil)

	requests := set.Unwatch("ns=2;s=Level")
	assertKinds(t, requests, RequestCancelSubscription)
	if requests[0].Subscription != sub {
		t.Fatalf("cancelled subscription = %#v", requests[0].Subscription)
	}
}

func assertKinds(t *testing.T, requests []Request, kinds ...RequestKind) {
	t.Helper()
	if len(requests) != len(kinds) {
		t.Fatalf("request count = %d, want %d: %#v", len(requests), len(kinds), requests)
	}
	for i, kind := range kinds {
		if requests[i].Kind != kind {
			t.Fatalf("request[%d] = %v, want %v", i, requests[i].Kind, kind)
		}
	}
}

type fakeSubscription struct{}

func (f *fakeSubscription) Cancel(ctx context.Context) error { return nil }
