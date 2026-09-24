package strava

import (
	"encoding/json"
	"fmt"

	"github.com/ilyasaftr/strava-auto-kudos/internal/kudos"
)

type FeedPage struct {
	Activities []kudos.Activity
	HasMore    bool
	Before     string
	Cursor     string
}

func ParseFeed(body []byte) (FeedPage, error) {
	var feed feedResponse
	if err := json.Unmarshal(body, &feed); err != nil {
		return FeedPage{}, fmt.Errorf("decode feed: %w", err)
	}
	page := FeedPage{
		Activities: make([]kudos.Activity, 0, len(feed.Entries)),
		HasMore:    feed.Pagination.HasMore,
	}
	for _, entry := range feed.Entries {
		switch entry.Entity {
		case "Activity":
			if act, ok := mapActivity(entry.Activity); ok {
				page.Activities = append(page.Activities, act)
			}
		case "GroupActivity":
			if entry.RowData == nil {
				continue
			}
			for _, raw := range entry.RowData.Activities {
				if act, ok := mapGroupActivity(raw); ok {
					page.Activities = append(page.Activities, act)
				}
			}
		}
		if entry.CursorData != nil {
			page.Before = entry.CursorData.UpdatedAt.String()
			page.Cursor = entry.CursorData.Rank.String()
		}
	}
	return page, nil
}

type feedResponse struct {
	Entries    []feedEntry `json:"entries"`
	Pagination struct {
		HasMore bool `json:"hasMore"`
	} `json:"pagination"`
}

type feedEntry struct {
	Entity     string        `json:"entity"`
	Activity   *feedActivity `json:"activity"`
	RowData    *rowData      `json:"rowData"`
	CursorData *cursorData   `json:"cursorData"`
}

type cursorData struct {
	UpdatedAt json.Number `json:"updated_at"`
	Rank      json.Number `json:"rank"`
}

type rowData struct {
	Activities []groupActivity `json:"activities"`
}

type feedActivity struct {
	ID                    json.Number `json:"id"`
	ActivityName          string      `json:"activityName"`
	OwnedByCurrentAthlete bool        `json:"ownedByCurrentAthlete"`
	Athlete               struct {
		AthleteID   json.Number `json:"athleteId"`
		AthleteName string      `json:"athleteName"`
	} `json:"athlete"`
	KudosAndComments struct {
		CanKudo bool `json:"canKudo"`
	} `json:"kudosAndComments"`
}

type groupActivity struct {
	ActivityID  json.Number `json:"activity_id"`
	Name        string      `json:"name"`
	AthleteID   json.Number `json:"athlete_id"`
	AthleteName string      `json:"athlete_name"`
	CanKudo     *bool       `json:"can_kudo"`
}

func mapActivity(raw *feedActivity) (kudos.Activity, bool) {
	if raw == nil {
		return kudos.Activity{}, false
	}
	id, err := raw.ID.Int64()
	if err != nil || id == 0 {
		return kudos.Activity{}, false
	}
	athleteID, _ := raw.Athlete.AthleteID.Int64()
	return kudos.Activity{
		ID:      kudos.ActivityID(id),
		Name:    raw.ActivityName,
		CanKudo: raw.KudosAndComments.CanKudo,
		Owned:   raw.OwnedByCurrentAthlete,
		Athlete: kudos.Athlete{
			ID:   kudos.AthleteID(athleteID),
			Name: raw.Athlete.AthleteName,
		},
	}, true
}

func mapGroupActivity(raw groupActivity) (kudos.Activity, bool) {
	id, err := raw.ActivityID.Int64()
	if err != nil || id == 0 {
		return kudos.Activity{}, false
	}
	athleteID, _ := raw.AthleteID.Int64()
	can := true
	if raw.CanKudo != nil {
		can = *raw.CanKudo
	}
	return kudos.Activity{
		ID:      kudos.ActivityID(id),
		Name:    raw.Name,
		CanKudo: can,
		Athlete: kudos.Athlete{
			ID:   kudos.AthleteID(athleteID),
			Name: raw.AthleteName,
		},
	}, true
}
