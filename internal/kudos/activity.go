package kudos

type ActivityID int64
type AthleteID int64

type Athlete struct {
	ID   AthleteID
	Name string
}

type Activity struct {
	ID      ActivityID
	Name    string
	CanKudo bool
	Owned   bool
	Athlete Athlete
}

func (a Activity) Eligible(viewer AthleteID) bool {
	if a.ID == 0 {
		return false
	}
	if a.Owned {
		return false
	}
	if !a.CanKudo {
		return false
	}
	if viewer != 0 && a.Athlete.ID == viewer {
		return false
	}
	return true
}
