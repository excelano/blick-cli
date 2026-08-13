package main

import (
	"encoding/json"
	"net/url"
	"time"
)

type Meeting struct {
	Subject   string
	Organizer string
	Location  string
	Start     time.Time
	End       time.Time
	IsOnline  bool
	IsAllDay  bool
	JoinURL   string
}

type calendarEvent struct {
	Subject   string `json:"subject"`
	Organizer struct {
		EmailAddress struct {
			Name string `json:"name"`
		} `json:"emailAddress"`
	} `json:"organizer"`
	Location struct {
		DisplayName string `json:"displayName"`
	} `json:"location"`
	Start struct {
		DateTime string `json:"dateTime"`
		TimeZone string `json:"timeZone"`
	} `json:"start"`
	End struct {
		DateTime string `json:"dateTime"`
		TimeZone string `json:"timeZone"`
	} `json:"end"`
	IsOnlineMeeting bool `json:"isOnlineMeeting"`
	IsAllDay        bool `json:"isAllDay"`
	OnlineMeeting   struct {
		JoinURL string `json:"joinUrl"`
	} `json:"onlineMeeting"`
}

func (e calendarEvent) toMeeting() Meeting {
	loc, err := time.LoadLocation(e.Start.TimeZone)
	if err != nil {
		loc = time.Local
	}
	start, _ := time.ParseInLocation("2006-01-02T15:04:05.0000000", e.Start.DateTime, loc)
	end, _ := time.ParseInLocation("2006-01-02T15:04:05.0000000", e.End.DateTime, loc)
	return Meeting{
		Subject:   flatten(e.Subject, " "),
		Organizer: e.Organizer.EmailAddress.Name,
		Location:  flatten(e.Location.DisplayName, ", "),
		Start:     start,
		End:       end,
		IsOnline:  e.IsOnlineMeeting,
		IsAllDay:  e.IsAllDay,
		JoinURL:   e.OnlineMeeting.JoinURL,
	}
}

func (g *GraphClient) NextMeeting() (*Meeting, error) {
	now := time.Now()
	endOfDay := now.Add(24 * time.Hour)

	query := url.Values{
		"startDateTime": {now.UTC().Format(time.RFC3339)},
		"endDateTime":   {endOfDay.UTC().Format(time.RFC3339)},
		"$top":          {"1"},
		"$orderby":      {"start/dateTime"},
		"$select":       {"subject,organizer,location,start,end,isOnlineMeeting,isAllDay,onlineMeeting"},
	}

	data, err := g.get("/me/calendarView", query)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []calendarEvent `json:"value"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	if len(result.Value) == 0 {
		return nil, nil
	}

	m := result.Value[0].toMeeting()
	return &m, nil
}

// TodaysMeetings returns every event on the user's calendar between local
// midnight today and local midnight tomorrow, ordered by start time. Past,
// current, and upcoming events are all included — the caller decides how to
// style them.
func (g *GraphClient) TodaysMeetings() ([]Meeting, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	end := start.Add(24 * time.Hour)

	query := url.Values{
		"startDateTime": {start.UTC().Format(time.RFC3339)},
		"endDateTime":   {end.UTC().Format(time.RFC3339)},
		"$orderby":      {"start/dateTime"},
		"$top":          {"100"},
		"$select":       {"subject,organizer,location,start,end,isOnlineMeeting,isAllDay,onlineMeeting"},
	}

	data, err := g.get("/me/calendarView", query)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []calendarEvent `json:"value"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	meetings := make([]Meeting, 0, len(result.Value))
	for _, e := range result.Value {
		meetings = append(meetings, e.toMeeting())
	}
	return meetings, nil
}
