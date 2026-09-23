package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Person struct {
	Name string `json:"name"`
}

// ChoreFrequency controls how often a Chore comes up for assignment.
type ChoreFrequency string

const (
	// Daily chores get a (possibly different) assignee every day.
	Daily ChoreFrequency = "daily"
	// Weekly chores are assigned to one person for the whole week, and only
	// show up on WeeklyDay.
	Weekly ChoreFrequency = "weekly"
)

// Chore describes a rotating household task. Icon is an emoji the frontend
// renders directly, so chore presentation lives in one place (here) instead
// of being duplicated in the UI.
type Chore struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Icon      string         `json:"icon"`
	Frequency ChoreFrequency `json:"frequency"`
	// WeeklyDay is the Day.Num (1=Sunday .. 7=Saturday) this chore is assigned
	// on. Only meaningful when Frequency is Weekly.
	WeeklyDay int `json:"weekly_day,omitempty"`
	// offset staggers this chore's rotation within housemates relative to
	// other chores sharing the same frequency, so they don't always land on
	// the same person.
	offset int
}

// Assignment pairs a Chore with the Person responsible for it on a given day.
type Assignment struct {
	Chore  Chore  `json:"chore"`
	Person Person `json:"person"`
}

// Day
// Num - sunday = 1, monday = 2, etc
// Assignments - one entry per chore, naming who's responsible that day
type Day struct {
	Num         int          `json:"num"`
	Assignments []Assignment `json:"assignments"`
}

// A Week's WeekNum shows which week of the rotation is in effect at runtime.
// Days is an array of the 7 days of the current week. TodayIdx is an index of the
// day of the week at runtime.
type Week struct {
	WeekNum  uint8  `json:"week_num"`
	Days     [7]Day `json:"days"`
	TodayIdx uint8  `json:"today_idx"`
}

// Response is a general structure used to provide
// access to common attributes that all API responses
// share.
type Response[T any] struct {
	Data      T      `json:"data"`
	TimeStamp string `json:"timestamp_utc"`
	// not sure if I need a status in the responses. at this point
	// I'm not doing any error handling so probably not atm.
	// Status    uint8  `json:"status"`
}

// Structure of incoming POST requests to the '/week' endpoint
type WeekRequest struct {
	Datetime time.Time `json:"datetime" time_format:"2006-01-02T15:04:05Z" time_utc:"1"`
}

const weekOffset = 0

// week 1 day 1 starts with evie
// week 2 day 1 Garrett
// week 3 day 1 Parker

// housemates is the shared rotation pool every chore draws from.
var housemates = []Person{
	{Name: "Evie"},
	{Name: "Garrett"},
	{Name: "Parker"},
}

// chores are the household tasks tracked day-to-day. Add a new chore by
// adding an entry here. Give a chore a distinct offset from others sharing
// its assignment day so they don't always land on the same person.
var chores = []Chore{
	{ID: "kitchen_cleaner", Name: "Kitchen", Icon: "🍽️", Frequency: Daily},
	{ID: "cat_litter", Name: "Litter Box", Icon: "🐈", Frequency: Weekly, WeeklyDay: 1, offset: 1}, // Sunday
}

// Returns the week index of the given Time (1-52). Weeks start on SUNDAYS,
// as opposed to ISOWeeks starting on Mondays.
func NonISOWeek(t time.Time) (year int, week int) {
	year = t.Year()
	// Find first day of the year
	startOfYear := time.Date(year, time.January, 1, 0, 0, 0, 0, t.Location())

	// Find the first Sunday of the year
	offset := (7 - int(startOfYear.Weekday())) % 7
	firstSunday := startOfYear.AddDate(0, 0, offset)
	if t.Before(firstSunday) {
		// Belongs to the last week of the previous year
		return NonISOWeek(time.Date(year-1, time.December, 31, 0, 0, 0, 0, t.Location()))
	}
	// Duration since first Sunday
	daysSince := int(t.Sub(firstSunday).Hours() / 24)
	week = (daysSince / 7) + 1 // +1 to make the first Sunday = week 1

	return
}

func calculateDays(weekNum int) [7]Day {
	calcDays := [7]Day{}
	numHousemates := len(housemates)
	for dayIdx := 0; dayIdx < 7; dayIdx++ {
		dayNum := dayIdx + 1
		var assignments []Assignment
		for _, chore := range chores {
			switch chore.Frequency {
			case Weekly:
				if dayNum != chore.WeeklyDay {
					continue
				}
				personIdx := (weekNum + chore.offset) % numHousemates
				assignments = append(assignments, Assignment{Chore: chore, Person: housemates[personIdx]})
			default: // Daily
				personIdx := (weekNum + dayIdx + chore.offset) % numHousemates
				assignments = append(assignments, Assignment{Chore: chore, Person: housemates[personIdx]})
			}
		}
		calcDays[dayIdx] = Day{
			Num:         dayNum,
			Assignments: assignments,
		}
	}
	return calcDays
}

func calculateWeek(aTime *time.Time) Response[Week] {
	_, week := NonISOWeek(*aTime)
	calcWeek := (week + weekOffset) % len(housemates)
	days := calculateDays(calcWeek)
	nowTime := time.Now().UTC()
	return Response[Week]{
		Data: Week{
			WeekNum: uint8(calcWeek) + 1,
			Days:    days,
			//use indexes 1-7 instead of 0-6
			TodayIdx: uint8(aTime.Weekday() + 1),
		},
		TimeStamp: nowTime.String(),
	}
}

// Adds 'access-control-allow-origin' header to response
// if client sends an Origin header
func handleOriginHeader(c *gin.Context) {
	originHeaderLen := len(c.Request.Header["Origin"])
	if originHeaderLen > 0 {
		c.Header("Access-Control-Allow-Origin", c.Request.Header["Origin"][0])
	}
}

// handles the CORS preflight (OPTIONS) request the browser sends ahead of
// the POST /week request, since it carries a Content-Type: application/json
// header and so doesn't qualify as a "simple request".
func handleWeekPreflight(c *gin.Context) {
	handleOriginHeader(c)
	c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type")
	c.Status(http.StatusNoContent)
}

// handler for getting the current week of data
func getCurrentWeek(c *gin.Context) {
	handleOriginHeader(c)
	now := time.Now()
	c.IndentedJSON(http.StatusOK, calculateWeek(&now))
}

// Expects a 'datetime' JSON property in RFC3339 format,
// i.e. `datetime: "2006-01-02T15:04:05Z"`
func getWeek(c *gin.Context) {
	handleOriginHeader(c)
	var req WeekRequest
	if err := c.ShouldBind(&req); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, calculateWeek(&req.Datetime))
}

func main() {
	router := gin.Default()
	router.GET("/week", getCurrentWeek)
	router.POST("/week", getWeek)
	router.OPTIONS("/week", handleWeekPreflight)

	router.Run("0.0.0.0:8008")
}
