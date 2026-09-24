package kudos

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type Feed interface {
	FollowingActivities(ctx context.Context) ([]Activity, error)
}

type Giver interface {
	GiveKudos(ctx context.Context, activityID ActivityID) error
}

type Poller struct {
	Feed      Feed
	Giver     Giver
	AthleteID AthleteID
	Delay     time.Duration
	Sleep     func(time.Duration)
	Logger    *slog.Logger
}

type TickResult struct {
	Fetched int
	Given   int
	Skipped int
}

func (p *Poller) Tick(ctx context.Context) (TickResult, error) {
	activities, err := p.Feed.FollowingActivities(ctx)
	if err != nil {
		return TickResult{}, err
	}

	result := TickResult{Fetched: len(activities)}

	sleep := p.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}

	for _, activity := range activities {
		if !activity.Eligible(p.AthleteID) {
			result.Skipped++
			continue
		}
		if err := p.Giver.GiveKudos(ctx, activity.ID); err != nil {
			return result, fmt.Errorf("kudos for activity %d: %w", activity.ID, err)
		}
		result.Given++
		p.log().Info("gave kudos",
			"activity_id", activity.ID,
			"athlete", activity.Athlete.Name,
			"name", activity.Name,
		)
		if p.Delay > 0 {
			sleep(p.Delay)
		}
	}
	return result, nil
}

func (p *Poller) log() *slog.Logger {
	if p.Logger != nil {
		return p.Logger
	}
	return slog.Default()
}
