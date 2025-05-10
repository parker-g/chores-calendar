package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Person struct {
	Name string `json:"name"`
}

type Day struct {
	Num    int          `json:"num"`
	Chores ChoresLineup `json:"chores"`
}

type ChoresLineup struct {
	KitchenCleaner Person `json:"kitchen_cleaner"`
	// TrashPerson    Person `json:"trash_person"`
}

type Week struct {
	Week int    `json:"week"`
	Days [7]Day `json:"days"`
}

// I can calculate weekdays / lineups dynamically. I think perhaps I will just need
// to supply an original set of values (all people + chores) then iterate over them
// for each day of the month using modulo

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
	// weekNum = 3
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

func calculateWeek() Week {
	timeSeconds := time.Now()
	_, week := timeSeconds.ISOWeek()
	calcWeek := (week + weekOffset) % 4
	days := calculateDays(calcWeek)
	return Week{
		// display Week as a 1-4 value instead of 0-3
		Week: calcWeek + 1,
		Days: days,
	}
}

// handler for getting the current week of data
func getWeek(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", c.Request.Header["Origin"][0])
	c.IndentedJSON(http.StatusOK, calculateWeek())
}

func main() {
	router := gin.Default()
	router.GET("/week", getWeek)

	router.Run("0.0.0.0:8008")
}
