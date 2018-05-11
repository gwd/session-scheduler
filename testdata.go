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
	profile := &UserProfile {
		RealName: fake.FullName(),
		Company: fake.Company(),
		Email: fake.EmailAddress(),
		Description: fake.Paragraphs(),
	}
	
	
	log.Printf("Creating test user %s %v", username, *profile)

	for _, err := NewUser(username, TestPassword, Event.VerificationCode, profile);
	           err != nil;
               _, err = NewUser(username, TestPassword, Event.VerificationCode, profile) {
     	if err == errUsernameExists {
			username = fake.UserName()
			log.Printf(" User exists!  Trying username %s instead", username)
			continue
		}
		log.Fatalf("Creating a test user: %v", err)
	}
}

func NewTestDiscussion(owner *User) {
	// Get a random owner
	title := fake.Title()
	desc := fake.Paragraphs()
	//title := randomdata.Noun()
	//desc := randomdata.Paragraph()

	if owner == nil {
		var err error
		owner, err = Event.Users.FindRandom()
		if err != nil {
			log.Fatalf("Getting a random user: %v", err)
		}
	}
	
	log.Printf("Creating discussion with owner %s, title %s, desc %s",
		owner.ID, title, desc)

	if _, err := NewDiscussion(owner, title, desc); err != nil {
		log.Fatal("Creating new discussion: %v", err)
	}
}

// Loop over all users and discussions, 50% of the time generating no interest,
// 50% of the time generating a random amount of interest between 1 and 100
func TestGenerateInterest() {
	for _, user := range Event.Users.GetUsers() {
		for _, disc := range Event.Discussions {
			r := rand.Intn(100)
			interest := 0
			switch {
			case r >= 40:
				interest = rand.Intn(100)
			case r >= 50:
				interest = 100
			}

			log.Printf("Setting uid %s interest in discussion %s to %d",
				user.Username, disc.Title, interest)
			if err := user.SetInterest(disc, interest); err != nil {
				log.Fatalf("Setting interest: %v", err)
			}
		}
	}
}

const (
	TestUsers = 8
	TestDisc = 6
	TestSlots = 4
)

func TestPopulate() {
	Event.Init(EventOptions{
		Slots: TestSlots,
		AdminPassword: "xenroot" })
	Event.TestMode = true
	for i := 0; i < TestUsers ; i++ {
		NewTestUser()
	}
	for i := 0; i < TestDisc ; i++ {
		NewTestDiscussion(nil)
	}
}
