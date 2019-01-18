package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type User struct {
	id     string `json:"id"`
	balance int `json:"balance"`
}

func getUserHandler(w http.ResponseWriter, r *http.Request) {

	users, err := store.GetUsers()

	// Everything else is the same as before
	userListBytes, err := json.Marshal(users)

	if err != nil {
		fmt.Println("getUserHandler: ", fmt.Errorf("Error: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(userListBytes)
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {

	user := User{}

	err := r.ParseForm()

	if err != nil {
		fmt.Println("createUserHandler 1: ", fmt.Errorf("Error: %v", err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fmt.Println("id is ", user.id+" balance is ", user.balance)
	user.id = r.Form.Get("id")
	int_balance, err := strconv.Atoi(r.Form.Get("balance"))
   if err != nil {
      //handle
   }
	user.balance = int_balance
	err = store.CreateUser(&user)
	if err != nil {
		fmt.Println("createUserHandler 2: ", err)
	}

	http.Redirect(w, r, "/assets/", http.StatusFound)
}