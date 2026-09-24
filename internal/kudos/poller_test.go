package kudos_test

import (
	"context"
	"testing"

	"github.com/ilyasaftr/strava-auto-kudos/internal/kudos"
)

type fakeFeed struct {
	activities []kudos.Activity
}

func (f fakeFeed) FollowingActivities(context.Context) ([]kudos.Activity, error) {
	return f.activities, nil
}

type fakeGiver struct {
	given []kudos.ActivityID
}

func (g *fakeGiver) GiveKudos(_ context.Context, activityID kudos.ActivityID) error {
	g.given = append(g.given, activityID)
	return nil
}

func TestGivesKudosToEligibleFriendsOnly(t *testing.T) {
	giver := &fakeGiver{}
	p := kudos.Poller{
		Feed: fakeFeed{activities: []kudos.Activity{
			{ID: 11, Name: "Outdoor walk", CanKudo: true, Athlete: kudos.Athlete{ID: 2, Name: "Ada"}},
			{ID: 12, Name: "Mine", Owned: true, Athlete: kudos.Athlete{ID: 1}},
		}},
		Giver:     giver,
		AthleteID: 1,
	}

	result, err := p.Tick(context.Background())
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if result.Given != 1 || result.Skipped != 1 {
		t.Fatalf("result %+v given %v", result, giver.given)
	}
	if len(giver.given) != 1 || giver.given[0] != 11 {
		t.Fatalf("given %v", giver.given)
	}
}

func TestSkipsOwnAndCannotKudo(t *testing.T) {
	giver := &fakeGiver{}
	p := kudos.Poller{
		Feed: fakeFeed{activities: []kudos.Activity{
			{ID: 1, Name: "Mine", Owned: true, Athlete: kudos.Athlete{ID: 99}},
			{ID: 2, Name: "Already", CanKudo: false, Athlete: kudos.Athlete{ID: 4}},
			{ID: 5, Name: "Locked", CanKudo: false, Athlete: kudos.Athlete{ID: 4}},
			{ID: 4, Name: "New", CanKudo: true, Athlete: kudos.Athlete{ID: 4, Name: "Ada"}},
		}},
		Giver:     giver,
		AthleteID: 99,
	}

	result, err := p.Tick(context.Background())
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if result.Given != 1 || result.Skipped != 3 {
		t.Fatalf("result %+v given %v", result, giver.given)
	}
	if len(giver.given) != 1 || giver.given[0] != 4 {
		t.Fatalf("given %v", giver.given)
	}
}
