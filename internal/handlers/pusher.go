package handlers

import (
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/pusher/pusher-http-go"
)

func (repo *DBRepo) PusherAuth(w http.ResponseWriter, r *http.Request) {
	userID := repo.App.Session.GetInt(r.Context(), "userID")

	u, err := repo.DB.GetUserById(userID)
	if err != nil {
		log.Println(err)
	}

	params, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
	}

	presenceData := pusher.MemberData{
		UserID: strconv.Itoa(userID),
		UserInfo: map[string]string{
			"name": u.FirstName,
			"id":   strconv.Itoa(userID),
		},
	}

	response, err := app.WsClient.AuthenticatePresenceChannel(params, presenceData)
	if err != nil {
		log.Println(err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(response)
}

func (repo *DBRepo) TestPusher(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]string)
	data["message"] = "Hello, world!"

	err := repo.App.WsClient.Trigger("public-channel", "test-event", data)
	if err != nil {
		log.Println(err)
	}
}
