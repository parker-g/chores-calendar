package main

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
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
	Chore     Chore  `json:"chore"`
	Person    Person `json:"person"`
	Completed bool   `json:"completed"`
}

// Day
// Num - sunday = 1, monday = 2, etc
// Assignments - one entry per chore, naming who's responsible that day
type Day struct {
	Num         int          `json:"num"`
	Date        string       `json:"date"`
	Assignments []Assignment `json:"assignments"`
}

// A Week's WeekNum shows which week of the rotation is in effect at runtime.
// Days is an array of the 7 days of the current week. TodayIdx is an index of the
// day of the week at runtime.
type Week struct {
	WeekNum uint8 `json:"week_num"`
	// WeekStart is the Sunday that starts this week, so the frontend can
	// derive the displayed date range from the same clock that determined
	// WeekNum instead of computing it independently from the browser's time.
	// It's a calendar date, not an instant, so it's a plain "YYYY-MM-DD"
	// string, not a time.Time: GET and POST /week resolve "now" in
	// different locations (server-local vs. UTC-normalized by gin's
	// binding), and shipping a zoned instant let the browser's local-time
	// rendering roll it back a day when the zones didn't line up. (Note:
	// encoding/json ignores gin's `time_format` tag on output, so a
	// time.Time field here would silently keep marshaling as full RFC3339
	// regardless of that tag.)
	WeekStart string `json:"week_start"`
	Days      [7]Day `json:"days"`
	TodayIdx  uint8  `json:"today_idx"`
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
	{Name: "Kristen"},
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

// calculateDays builds one week's assignments. absoluteWeek is the
// non-wrapped week count (see NonISOWeek) so that daily rotation can be
// keyed off actual elapsed days (absoluteWeek*7 + dayIdx): that keeps each
// day's assignee exactly one step past the previous day's, including across
// week boundaries, instead of resetting/jumping based on a week number
// that's already been wrapped modulo numHousemates.
func calculateDays(absoluteWeek int, weekStart time.Time) [7]Day {
	calcDays := [7]Day{}
	numHousemates := len(housemates)
	for dayIdx := 0; dayIdx < 7; dayIdx++ {
		dayNum := dayIdx + 1
		date := weekStart.AddDate(0, 0, dayIdx).Format("2006-01-02")
		var assignments []Assignment
		for _, chore := range chores {
			var personIdx int
			switch chore.Frequency {
			case Weekly:
				if dayNum != chore.WeeklyDay {
					continue
				}
				personIdx = (absoluteWeek + chore.offset) % numHousemates
			default: // Daily
				personIdx = (absoluteWeek*7 + dayIdx + chore.offset) % numHousemates
			}
			completed, err := isCompleted(chore.ID, date)
			if err != nil {
				log.Printf("completion lookup failed for chore_id=%s date=%s: %v", chore.ID, date, err)
				completed = false
			}
			assignments = append(assignments, Assignment{
				Chore:     chore,
				Person:    housemates[personIdx],
				Completed: completed,
			})
		}
		calcDays[dayIdx] = Day{
			Num:         dayNum,
			Date:        date,
			Assignments: assignments,
		}
	}
	return calcDays
}

// startOfWeek returns midnight on the Sunday that starts aTime's week.
func startOfWeek(aTime time.Time) time.Time {
	y, m, d := aTime.Date()
	dayStart := time.Date(y, m, d, 0, 0, 0, 0, aTime.Location())
	return dayStart.AddDate(0, 0, -int(dayStart.Weekday()))
}

func calculateWeek(aTime *time.Time) Response[Week] {
	_, week := NonISOWeek(*aTime)
	absoluteWeek := week + weekOffset
	calcWeek := absoluteWeek % len(housemates)
	weekStart := startOfWeek(*aTime)
	days := calculateDays(absoluteWeek, weekStart)
	nowTime := time.Now().UTC()
	return Response[Week]{
		Data: Week{
			WeekNum:   uint8(calcWeek) + 1,
			WeekStart: weekStart.Format("2006-01-02"),
			Days:      days,
			//use indexes 1-7 instead of 0-6
			TodayIdx: uint8(aTime.Weekday() + 1),
		},
		TimeStamp: nowTime.String(),
	}
}

// allowedOrigins holds the set of origins permitted to make cross-origin
// requests to this API, loaded from the ALLOWED_ORIGINS env var at startup.
var allowedOrigins map[string]bool

// loadAllowedOrigins parses a comma-separated list of origins (e.g.
// "https://chores.example.com,http://localhost:8008") from the
// ALLOWED_ORIGINS env var into a lookup set.
func loadAllowedOrigins() map[string]bool {
	origins := map[string]bool{}
	raw := os.Getenv("ALLOWED_ORIGINS")
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins[o] = true
		}
	}
	return origins
}

// Adds an 'access-control-allow-origin' header to the response, but only
// when the client's Origin matches an entry in allowedOrigins — unlike
// reflecting any Origin verbatim, this stops arbitrary third-party websites
// from being granted cross-origin access.
//
// Some browsers omit the Origin header under stricter privacy modes (e.g.
// private/incognito browsing) even for genuine cross-origin fetches. When
// that happens, this falls back to deriving the origin from the Referer
// header instead. Referer is a weaker signal than Origin — it can be
// stripped by privacy tools/extensions or a strict Referrer-Policy — but is
// good enough for this app's threat model, and only ever narrows access
// (checked against the same allowedOrigins set) rather than widening it.
func handleOriginHeader(c *gin.Context) {
	origin := c.Request.Header.Get("Origin")
	if origin == "" {
		if referer := c.Request.Header.Get("Referer"); referer != "" {
			if refURL, err := url.Parse(referer); err == nil && refURL.Scheme != "" && refURL.Host != "" {
				origin = refURL.Scheme + "://" + refURL.Host
			}
		}
	}
	if origin != "" && allowedOrigins[origin] {
		c.Header("Access-Control-Allow-Origin", origin)
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
	allowedOrigins = loadAllowedOrigins()
	if len(allowedOrigins) == 0 {
		log.Println("warning: ALLOWED_ORIGINS is unset/empty; no cross-origin requests will be permitted")
	}

	if err := initCompletionsDB("chores.db"); err != nil {
		panic(err)
	}
	defer db.Close()

	router := gin.Default()
	router.GET("/week", getCurrentWeek)
	router.POST("/week", getWeek)
	router.OPTIONS("/week", handleWeekPreflight)

	router.POST("/completions/toggle", handleToggleCompletion)
	router.OPTIONS("/completions/toggle", handleCompletionsPreflight)

	router.Run("0.0.0.0:8008")
}
