const BASE_API_URL = "http://localhost:8008"
const LOOKAHEAD_WEEKS = 3;

const DAY_NAMES = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];

// thanks Stackoverflow <3
Date.prototype.GetFirstDayOfWeek = function() {
    return (new Date(this.setDate(this.getDate() - this.getDay())));
}

Date.prototype.GetLastDayOfWeek = function() {
    return (new Date(this.setDate(this.getDate() - this.getDay() + 6)));
}

/** Sunday of the week that is `offsetWeeks` weeks from the current one. */
function getWeekStart(offsetWeeks) {
    const d = new Date();
    d.setDate(d.getDate() + offsetWeeks * 7);
    return d.GetFirstDayOfWeek();
}

function shortDate(date) {
    return date.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

/** Fetches week data `offsetWeeks` weeks from now (0 = current week). */
function fetchWeekData(offsetWeeks) {
    const isCurrent = offsetWeeks === 0;
    const request = isCurrent
        ? fetch(BASE_API_URL + "/week")
        : fetch(BASE_API_URL + "/week", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ datetime: getWeekStart(offsetWeeks).toISOString() }),
        });

    return request
        .then((response) => {
            if (!response.ok) {
                throw new Error(`HTTP Error! status: ${response.status}`);
            }
            return response.json();
        })
        .then((jsonData) => {
            const data = jsonData['data'];
            return {
                weekNum: data['week_num'],
                days: data['days'],
                // "today" only means something for the current week
                todayIdx: isCurrent ? parseInt(data['today_idx']) : null,
                weekStart: getWeekStart(offsetWeeks),
            };
        });
}

fetchWeekData(0).then((week) => {
    displayWeekNum(week.weekNum);
    displayWeekSubheader(week.weekStart);
    renderWeekRow(document.getElementById("currentWeekGrid"), week);
    displayTodaySpotlight(week.days, week.todayIdx);

    // on narrow viewports the week row is a horizontally-scrollable
    // carousel, so bring today's card into view instead of leaving the
    // user parked on Sunday.
    document.getElementById("today")?.scrollIntoView({ inline: "center", block: "nearest" });
});

setUpLookaheadToggle();

function initialsForName(name) {
    return name.trim().charAt(0).toUpperCase();
}

function displayWeekSubheader(weekStart) {
    const targetContainer = document.getElementsByClassName("week-range").item(0);
    const weekEnd = new Date(weekStart);
    weekEnd.setDate(weekStart.getDate() + 6);
    targetContainer.textContent = `${shortDate(weekStart)} - ${shortDate(weekEnd)}`;
}

function displayWeekNum(weekNum) {
    const targetContainer = document.getElementsByClassName("week-badge").item(0);
    targetContainer.textContent = `Week ${weekNum}`;
}

function displayTodaySpotlight(choresData, todayIdx) {
    const targetContainer = document.getElementsByClassName("today-spotlight").item(0);
    targetContainer.innerHTML = "";

    const todayChores = Array.isArray(choresData)
        ? choresData.find((day) => day.num === todayIdx)
        : undefined;

    if (!todayChores || !Array.isArray(todayChores.assignments)) {
        targetContainer.style.display = "none";
        return;
    }

    todayChores.assignments.forEach((assignment) => {
        const name = assignment.person.name;

        const item = document.createElement("div");
        item.classList.add("spotlight-item");

        const avatar = document.createElement("div");
        avatar.classList.add("spotlight-avatar");
        avatar.textContent = assignment.chore.icon || initialsForName(name);

        const textWrap = document.createElement("div");
        textWrap.classList.add("spotlight-text");

        const label = document.createElement("span");
        label.classList.add("spotlight-label");
        label.textContent = assignment.chore.name;

        const spotlightName = document.createElement("span");
        spotlightName.classList.add("spotlight-name");
        spotlightName.textContent = name;

        textWrap.appendChild(label);
        textWrap.appendChild(spotlightName);

        item.appendChild(avatar);
        item.appendChild(textWrap);
        targetContainer.appendChild(item);
    });
}

/**
 * Renders one week's 7 day cards into `gridContainer`.
 * `week` is { days, todayIdx, weekStart }; todayIdx may be null for weeks
 * where no day should be marked past/today (i.e. lookahead weeks).
 */
function renderWeekRow(gridContainer, week) {
    gridContainer.innerHTML = "";

    const { days, todayIdx, weekStart } = week;
    if (!Array.isArray(days)) {
        throw new Error("Expected chores days to be an array.")
    }

    days.forEach((choreDay) => {
        const dayDiv = document.createElement("div");
        dayDiv.classList.add("day");

        const dayHeader = document.createElement("p");
        dayHeader.classList.add("day-name");
        dayHeader.textContent = getDay(choreDay.num);
        dayDiv.appendChild(dayHeader);

        const dayDate = document.createElement("p");
        dayDate.classList.add("day-date");
        const dateForDay = new Date(weekStart);
        dateForDay.setDate(weekStart.getDate() + (choreDay.num - 1));
        dayDate.textContent = shortDate(dateForDay);
        dayDiv.appendChild(dayDate);

        buildDayAssignmentsDiv(dayDiv, choreDay);
        gridContainer.appendChild(dayDiv);

        // hover doesn't exist on touch devices, so tapping toggles the
        // same "focused" look that :hover gives on desktop.
        dayDiv.addEventListener("click", () => {
            dayDiv.classList.toggle("focused");
        })

        if (todayIdx !== null) {
            if (choreDay.num < todayIdx) {
                dayDiv.classList.add("past");
            } else if (choreDay.num === todayIdx) {
                dayDiv.id = 'today';
            }
        }
    });
}

function buildDayAssignmentsDiv(dayContainer, choreDay) {
    const assignmentsList = document.createElement("div");
    assignmentsList.classList.add("day-assignments");

    (choreDay.assignments || []).forEach((assignment) => {
        const name = assignment.person.name;

        const row = document.createElement("p");
        row.classList.add("assignment-row");

        const icon = document.createElement("span");
        icon.classList.add("assignment-icon");
        icon.textContent = assignment.chore.icon || "";

        const shortName = document.createElement("span");
        shortName.classList.add("assignment-name-short");
        shortName.textContent = initialsForName(name);

        const fullName = document.createElement("span");
        fullName.classList.add("assignment-name-full");
        fullName.textContent = name;

        row.appendChild(icon);
        row.appendChild(shortName);
        row.appendChild(fullName);
        assignmentsList.appendChild(row);
    });

    dayContainer.appendChild(assignmentsList);
}

function getDay(day) {
    if (day >= 1 && day <= 7) {
        return DAY_NAMES[day - 1];
    }
    throw new Error(`Bad day value, ${day}`);
}

/** Wires up the "look ahead" toggle, lazily fetching the next weeks on first use. */
function setUpLookaheadToggle() {
    const toggle = document.getElementById("lookaheadToggle");
    const weeksContainer = document.getElementById("weeksContainer");
    let lookaheadLoaded = false;

    const syncLookaheadState = () => {
        weeksContainer.classList.toggle("show-lookahead", toggle.checked);

        if (toggle.checked && !lookaheadLoaded) {
            lookaheadLoaded = true;
            loadLookaheadWeeks(weeksContainer);
        }
    };

    toggle.addEventListener("change", syncLookaheadState);

    // the browser can restore a checkbox's checked state across a plain
    // refresh without firing "change", so sync once on load too.
    syncLookaheadState();
}

function loadLookaheadWeeks(weeksContainer) {
    const offsets = Array.from({ length: LOOKAHEAD_WEEKS }, (_, i) => i + 1);

    Promise.all(offsets.map(fetchWeekData))
        .then((weeks) => {
            weeks.forEach((week) => {
                weeksContainer.appendChild(buildLookaheadWeekRow(week));
            });
        })
        .catch((err) => {
            console.error("Failed to load lookahead weeks:", err);
        });
}

function buildLookaheadWeekRow(week) {
    const row = document.createElement("div");
    row.classList.add("week-row", "lookahead");

    const label = document.createElement("p");
    label.classList.add("week-row-label");
    const weekEnd = new Date(week.weekStart);
    weekEnd.setDate(week.weekStart.getDate() + 6);
    label.textContent = `Week ${week.weekNum} · ${shortDate(week.weekStart)} - ${shortDate(weekEnd)}`;
    row.appendChild(label);

    const grid = document.createElement("div");
    grid.classList.add("choresList");
    row.appendChild(grid);

    renderWeekRow(grid, week);

    return row;
}
