const BASE_API_URL = "http://localhost:8008"

// thanks Stackoverflow <3
Date.prototype.GetFirstDayOfWeek = function() {
    return (new Date(this.setDate(this.getDate() - this.getDay())));
}

Date.prototype.GetLastDayOfWeek = function() {
    return (new Date(this.setDate(this.getDate() - this.getDay() +6)));
}

fetch(BASE_API_URL + "/week")
    .then( response => {
        if (!response.ok) {
            throw new Error(`HTTP Error! status: ${response.status}`);
        }
        return response.json();
    })
    .then( jsonData => {
        const data = jsonData['data'];
        const weekNum = data['week_num'];
        const choresData = data['days'];
        const todayIdx = data['today_idx'];
        displayWeekNum(weekNum);
        displayWeekSubheader();
        displayChores(choresData, todayIdx);
    })

function displayWeekSubheader() {
    const targetContainer = document.getElementsByClassName("week-subheader").item(0);
    targetContainer.innerHTML = '';
    
    const curr = new Date(); // right now
    const lastDay = curr.GetLastDayOfWeek().toUTCString()
        .split(" ")
        .slice(0, 3)
        .join(" ");
    const firstDay = curr.GetFirstDayOfWeek().toUTCString()
        .split(" ")
        .slice(0, 3)
        .join(" ");

    const realWeek = document.createElement("h2");
    realWeek.textContent = `${firstDay} - ${lastDay}`; 
    targetContainer.appendChild(realWeek);
}

function displayWeekNum(weekNum) {
    const targetContainer = document.getElementsByClassName("week").item(0);
    targetContainer.innerHTML = "";
    
    const weekTitle = document.createElement("h1");
    weekTitle.textContent = `Week: ${weekNum}`
    targetContainer.appendChild(weekTitle);
}

function displayChores(choresData, todayIdx) {
    const targetContainer = document.getElementsByClassName("choresList")[0];
    targetContainer.innerHTML = "";
    
    // todayIdx starts at 1
    let count = 1;
    if (Array.isArray(choresData)) {
        choresData.forEach(( choreDay )  => { 
            const dayDiv = document.createElement("div");
            const dayHeader = document.createElement("h2");
            dayHeader.textContent = getDay(choreDay.num);
            dayDiv.appendChild(dayHeader);
            dayDiv.classList.add("day");
            targetContainer.appendChild(dayDiv);
            dayDiv.addEventListener("click", ( event ) => {
                console.log("clicked");
                handleClickDay(dayDiv, choreDay, targetContainer); 
            })
            if (count == todayIdx) {
                dayDiv.id = 'today';
                buildDayChoresDiv(dayDiv, choreDay);
            }
            count += 1;
        });
    } else {
        throw new Error("Expected chores days to be an array.")
    }
}

function handleClickDay(dayContainer, choreDay, parentContainer) {
    // check if div of chores is already presented - if so,
    // hide / delete it.
    console.log("handling ClickDay for dayContainer: " + dayContainer.textContent);
    const dayChoresHeaders = dayContainer.querySelectorAll("h3")
    if (dayChoresHeaders.length > 0) {
        console.log("dayChoresHeaders: " + dayChoresHeaders);
        dayChoresHeaders.forEach(( header ) => {
            dayContainer.removeChild(header);
        })
        dayContainer.classList.remove("expanded");

    } else {
        // otherwise, build the div of chores + display it (with transition)
        console.log("attempting to build dayChoresDiv for day: " + dayContainer.textContent);
        buildDayChoresDiv(dayContainer, choreDay);
    }

}


function buildDayChoresDiv(dayContainer, choreDay) {
    // kitchen cleaner for the day
    const dayKitchenCleaner = document.createElement("h3");
    dayKitchenCleaner.textContent = `Kitchen: ${choreDay.chores.kitchen_cleaner.name}`;
    dayContainer.appendChild(dayKitchenCleaner);
    dayContainer.classList.add("expanded");
    void dayContainer.offsetHeight;
}


function getDay(day) {
    switch (day) {
        case 1:
            return "Sunday"
        case 2:
            return "Monday"
        case 3:
            return "Tuesday"
        case 4:
            return "Wednesday"
        case 5:
            return "Thursday"
        case 6:
            return "Friday"
        case 7:
            return "Saturday"
        default:
            throw new Error(`Bad day value, ${day}`);
    }
}
