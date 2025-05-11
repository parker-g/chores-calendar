const BASE_API_URL = "http://chores:8008"

fetch(BASE_API_URL + "/week")
    .then( response => {
        if (!response.ok) {
            throw new Error(`HTTP Error! status: ${response.status}`);
        }
        return response.json();
    })
    .then( jsonData => {
        const weekNum = jsonData['week_num'];
        const choresData = jsonData['days'];
        displayWeekNum(weekNum);
        displayChores(choresData);
    })

function displayWeekNum(weekNum) {
    const targetContainer = document.getElementById("week");
    targetContainer.innerHTML = "";
    
    const weekTitle = document.createElement("h1");
    weekTitle.textContent = `Week: ${weekNum}`
    targetContainer.appendChild(weekTitle);
}

function displayChores(choresData) {
    const targetContainer = document.getElementById("choresList");
    targetContainer.innerHTML = "";
    
    if (Array.isArray(choresData)) {
        choresData.forEach( choreDay => { 
            const dayDiv = document.createElement("div");
            const dayHeader = document.createElement("h2");
            const dayKitchenCleaner = document.createElement("h3");
            dayHeader.textContent = getDay(choreDay.num);
            dayKitchenCleaner.textContent = `Kitchen: ${choreDay.chores.kitchen_cleaner.name}`;
            dayDiv.appendChild(dayHeader);
            dayDiv.appendChild(dayKitchenCleaner);
            targetContainer.appendChild(dayDiv);
        });
    } else {
        throw new Error("Expected chores days to be an array.")
    }
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