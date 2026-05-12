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
}

func (g *GraphClient) NextMeeting() (*Meeting, error) {
	now := time.Now()
	endOfDay := now.Add(24 * time.Hour)

	query := url.Values{
		"startDateTime": {now.UTC().Format(time.RFC3339)},
		"endDateTime":   {endOfDay.UTC().Format(time.RFC3339)},
		"$top":          {"1"},
		"$orderby":      {"start/dateTime"},
		"$select":       {"subject,organizer,location,start,end,isOnlineMeeting"},
	}

	data, err := g.get("/me/calendarView", query)
	if err != nil {
		return nil, err
	}

	var result struct {
		Value []struct {
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
		} `json:"value"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	if len(result.Value) == 0 {
		return nil, nil
	}

	e := result.Value[0]

	loc, err := time.LoadLocation(e.Start.TimeZone)
	if err != nil {
		loc = time.Local
	}
	start, _ := time.ParseInLocation("2006-01-02T15:04:05.0000000", e.Start.DateTime, loc)
	end, _ := time.ParseInLocation("2006-01-02T15:04:05.0000000", e.End.DateTime, loc)

	return &Meeting{
		Subject:   e.Subject,
		Organizer: e.Organizer.EmailAddress.Name,
		Location:  e.Location.DisplayName,
		Start:     start,
		End:       end,
		IsOnline:  e.IsOnlineMeeting,
	}, nil
}
