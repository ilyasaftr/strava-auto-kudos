package strava_test

import (
	"testing"

	"github.com/ilyasaftr/strava-auto-kudos/internal/strava"
)

func TestParseFeedMapsFriendAndOwnActivities(t *testing.T) {
	body := []byte(`{
	  "entries": [
	    {
	      "entity": "Activity",
	      "activity": {
	        "id": "11",
	        "activityName": "Outdoor walk",
	        "ownedByCurrentAthlete": false,
	        "athlete": {"athleteId": "99", "athleteName": "Ada"},
	        "kudosAndComments": {"canKudo": true, "hasKudoed": false}
	      }
	    },
	    {
	      "entity": "Activity",
	      "activity": {
	        "id": "22",
	        "activityName": "Mine",
	        "ownedByCurrentAthlete": true,
	        "athlete": {"athleteId": "140248321", "athleteName": "Me"},
	        "kudosAndComments": {"canKudo": false, "hasKudoed": false}
	      }
	    }
	  ]
	}`)
	page, err := strava.ParseFeed(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Activities) != 2 {
		t.Fatalf("len %d", len(page.Activities))
	}
	if page.Activities[0].ID != 11 || !page.Activities[0].CanKudo || page.Activities[0].Owned {
		t.Fatalf("friend %+v", page.Activities[0])
	}
	if page.Activities[1].ID != 22 || !page.Activities[1].Owned {
		t.Fatalf("own %+v", page.Activities[1])
	}
}

func TestParseFeedReadsPaginationCursor(t *testing.T) {
	body := []byte(`{
	  "entries": [{
	    "entity": "Activity",
	    "cursorData": {"updated_at": 1788954445, "rank": 1789300119191},
	    "activity": {
	      "id": "11",
	      "activityName": "Walk",
	      "athlete": {"athleteId": "99", "athleteName": "Ada"},
	      "kudosAndComments": {"canKudo": true, "hasKudoed": false}
	    }
	  }],
	  "pagination": {"hasMore": true}
	}`)
	page, err := strava.ParseFeed(body)
	if err != nil {
		t.Fatal(err)
	}
	if !page.HasMore || page.Before != "1788954445" || page.Cursor != "1789300119191" {
		t.Fatalf("page %+v", page)
	}
}
