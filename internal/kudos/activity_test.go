package kudos_test

import (
	"testing"

	"github.com/ilyasaftr/strava-auto-kudos/internal/kudos"
)

func TestEligibleFriendActivity(t *testing.T) {
	activity := kudos.Activity{
		ID:      11,
		CanKudo: true,
		Athlete: kudos.Athlete{ID: 2, Name: "Ada"},
	}
	if !activity.Eligible(1) {
		t.Fatal("expected eligible")
	}
}

func TestEligibleRejectsOwnAndLocked(t *testing.T) {
	viewer := kudos.AthleteID(99)
	cases := []kudos.Activity{
		{ID: 0, CanKudo: true, Athlete: kudos.Athlete{ID: 1}},
		{ID: 1, Owned: true, Athlete: kudos.Athlete{ID: viewer}},
		{ID: 3, CanKudo: false, Athlete: kudos.Athlete{ID: 4}},
		{ID: 4, CanKudo: true, Athlete: kudos.Athlete{ID: viewer}},
	}
	for _, activity := range cases {
		if activity.Eligible(viewer) {
			t.Fatalf("expected skip %+v", activity)
		}
	}
}
