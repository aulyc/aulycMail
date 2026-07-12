package sync

import "testing"

func TestHighestLocalUID(t *testing.T) {
	tests := []struct {
		name      string
		localUIDs []uint32
		wantUID   uint32
		wantFound bool
	}{
		{
			name:      "empty mailbox requires full UID search",
			localUIDs: nil,
			wantFound: false,
		},
		{
			name:      "uses local message high-water mark",
			localUIDs: []uint32{21869, 21861, 21868},
			wantUID:   21869,
			wantFound: true,
		},
		{
			name:      "does not assume UID order",
			localUIDs: []uint32{9, 42, 17},
			wantUID:   42,
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUID, gotFound := highestLocalUID(tt.localUIDs)
			if gotUID != tt.wantUID || gotFound != tt.wantFound {
				t.Fatalf("highestLocalUID() = (%d, %v), want (%d, %v)", gotUID, gotFound, tt.wantUID, tt.wantFound)
			}
		})
	}
}
