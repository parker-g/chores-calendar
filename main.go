package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Person struct {
	Name string `json:"name"`
}

// Day
// Num - sunday = 0, monday = 1, etc
// Chores - A ChoresLineup
type Day struct {
	Num    int          `json:"num"`
	Chores ChoresLineup `json:"chores"`
}

// A collection of Persons paired to their chore for the day.
// i.e. "kitchen_cleaner: parker"
type ChoresLineup struct {
	KitchenCleaner Person `json:"kitchen_cleaner"`
	// TrashPerson    Person `json:"trash_person"`
}

// A Week's WeekNum shows which week of the 4 week rotation is in effect at runtime
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

const weekOffset = 1

// week 1 day 1 starts with evie
// week 2 day 1 Josie
// week 3 day 1 Garrett
// week 4 day 1 Parker

var choreCandidates = []Person{
	{Name: "Evie"},
	{Name: "Josie"},
	{Name: "Garrett"},
	{Name: "Parker"},
}

func calculateDays(weekNum int) [7]Day {
	calcDays := [7]Day{}
	for i := weekNum; i < weekNum+7; i++ {
		personIdx := i % 4
		dayIdx := i - weekNum
		dishDoer := choreCandidates[personIdx]
		lineup := ChoresLineup{
			KitchenCleaner: dishDoer,
		}
		newDay := Day{
			// output day num as 1-7 instead of 0-6
			Num:    dayIdx + 1,
			Chores: lineup,
		}
		calcDays[dayIdx] = newDay
	}
	return calcDays
}

func calculateWeek() Response[Week] {
	timeSeconds := time.Now()
	_, week := timeSeconds.ISOWeek()
	calcWeek := (week + weekOffset) % 4
	days := calculateDays(calcWeek)
	nowTime := time.Now().UTC()
	return Response[Week]{
		Data: Week{
			WeekNum:  uint8(calcWeek) + 1,
			Days:     days,
			TodayIdx: uint8(nowTime.Weekday()),
		},
		TimeStamp: nowTime.String(),
	}
}

// handler for getting the current week of data
func getWeek(c *gin.Context) {
	originHeaderLen := len(c.Request.Header["Origin"])
	if originHeaderLen > 0 {
		c.Header("Access-Control-Allow-Origin", c.Request.Header["Origin"][0])
	}
	c.IndentedJSON(http.StatusOK, calculateWeek())
}

func main() {
	router := gin.Default()
	router.GET("/week", getWeek)

	router.Run("0.0.0.0:8008")
}
