package main

import (
	"log"
	"math/rand"
	"time"

	"github.com/icrowley/fake"
	//"github.com/Pallinder/go-randomdata"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// This password should always be suitable 
const TestPassword = "xenuser"

func NewTestUser() {
	username := fake.UserName()
	email := fake.EmailAddress()
	//username := randomdata.SillyName()
	//email := randomdata.Email()
	
	
	log.Printf("Creating test user %s %s", username, email)

	if _, err := NewUser(username, email, TestPassword); err != nil {
		log.Fatalf("Creating a test user: %v", err)
	}
}

func NewTestDiscussion(owner UserID) {
	// Get a random owner
	title := fake.Title()
	desc := fake.Paragraphs()
	//title := randomdata.Noun()
	//desc := randomdata.Paragraph()

	if owner == "" {
		user, err := Event.Users.FindRandom()
		if err != nil {
			log.Fatalf("Getting a random user: %v", err)
		}
		owner = user.ID
	}
	
	log.Printf("Creating discussion with owner %s, title %s, desc %s",
		owner, title, desc)

	if _, err := NewDiscussion(owner, title, desc); err != nil {
		log.Fatal("Creating new discussion: %v", err)
	}
}
