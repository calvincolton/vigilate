package handlers

import (
	"fmt"
	"log"
	"strconv"
	"time"
)

type job struct {
	HostServiceID int
}

func (j job) Run() {
	Repo.ScheduledCheck(j.HostServiceID)
}

func (repo *DBRepo) StartMonitoring() {
	if app.PreferenceMap["monitoring_live"] == "1" {
		// trigger a message to broadcast to all clients letting them now that the app is starting to monitor
		data := make(map[string]string)
		data["message"] = "Monitor is starting..."
		err := app.WsClient.Trigger("public-channel", "app-starting", data)
		if err != nil {
			log.Println(err)
		}

		// get all of the services that we want to monitor
		servicesToMonitor, err := repo.DB.GetServicesToMonitor()
		if err != nil {
			log.Println(err)
		}

		// range through the services
		for _, s := range servicesToMonitor {
			log.Println("Service to monitor on", s.HostName, "is", s.Service.ServiceName)
			// get the schedule unit and number
			var sch string
			if s.ScheduleUnit == "d" {
				sch = fmt.Sprintf("@every %d%s", s.ScheduleNumber*24, "h")
			} else {
				sch = fmt.Sprintf("@every %d%s", s.ScheduleNumber, s.ScheduleUnit)
			}

			// create a job
			var j job
			j.HostServiceID = s.ID
			scheduleID, err := app.Scheduler.AddJob(sch, j)
			if err != nil {
				log.Println(err)
			}

			// save the id of the job so we can start/stop it
			app.MonitorMap[s.ID] = scheduleID

			// broadcast over websockets the fact that the service is scheduled
			payload := make(map[string]string)
			payload["message"] = "scheduling"
			payload["host_service_id"] = strconv.Itoa(s.ID)
			yearOne := time.Date(0001, 11, 17, 20, 34, 58, 65138838, time.UTC)
			if app.Scheduler.Entry(app.MonitorMap[s.ID]).Next.After(yearOne) {
				payload["next_run"] = app.Scheduler.Entry(app.MonitorMap[s.ID]).Next.Format("2006-01-02 3:04:05 PM")
			} else {
				payload["next_run"] = "Pending..."
			}
			payload["host"] = s.HostName
			payload["service"] = s.Service.ServiceName
			if s.LastCheck.After(yearOne) {
				payload["last_run"] = s.LastCheck.Format("2006-01-02 3:04:05 PM")
			} else {
				payload["last_run"] = "Pending..."
			}
			payload["schedule"] = fmt.Sprintf("@every %d%s", s.ScheduleNumber, s.ScheduleUnit)

			err = app.WsClient.Trigger("public-channel", "next-run-event", payload)
			if err != nil {
				log.Println(err)
			}

			err = app.WsClient.Trigger("public-channel", "schedule-change-event", payload)
			if err != nil {
				log.Println(err)
			}
		}
	}
}
