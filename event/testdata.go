package event

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
	user := User{
		Username:    fake.UserName(),
		IsVerified:  false,
		RealName:    fake.FullName(),
		Company:     fake.Company(),
		Email:       fake.EmailAddress(),
		Description: fake.Paragraphs(),
	}

	log.Printf("Creating test user %v", user)

	for _, err := NewUser(TestPassword, user); err != nil; _, err = NewUser(TestPassword, user) {
		// Just keep trying random usernames until we get a new one
		if err == errUsernameExists {
			user.Username = fake.UserName()
			log.Printf(" User exists!  Trying username %s instead", user.Username)
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
		owner, err = event.Users.FindRandom()
		if err != nil {
			log.Fatalf("Getting a random user: %v", err)
		}
	}

	var disc *Discussion

	for {
		var err error
		log.Printf("Creating discussion with owner %s, title %s, desc %s",
			owner.ID, title, desc)

		disc, err = NewDiscussion(owner, title, desc)
		switch err {
		case errTitleExists:
			title = fake.Title()
			continue
		case errTooManyDiscussions:
			// We could try a new user; but we don't want to loop forever
			// if all users are full of their quota, and we don't want to spend
			// time detecting that condition.  Just silently fail in that case.
			err = nil
			break
		}
		if err == nil {
			break
		}
		log.Fatalf("Creating new discussion: %v", err)
	}

	// Only 25% of discussions have constraints
	if disc != nil && rand.Intn(4) == 0 {
		// Make a continuous range where it's not schedulable
		start := rand.Intn(len(disc.PossibleSlots))
		end := rand.Intn(len(disc.PossibleSlots)-start) + 1
		if start != 0 || end != len(disc.PossibleSlots) {
			for i := start; i < end; i++ {
				disc.PossibleSlots[i] = false
			}
		}
	}
}

// Try to emulate "realistic" interest, where people will be like one another.
// - Create four "unique" people at the beginning, with random interests
// - Afterwards, choose someone randomly to emulate 90% of the time.
// - When emulating somebody, choose like them 7/8 times
func TestGenerateInterest() {
	handled := []*User{}
	for _, user := range event.Users.GetUsers() {
		var model *User
		// Create 4 random "models" at first; after that, 10% are random
		if len(handled) > 4 && rand.Intn(10) != 0 {
			model = handled[rand.Intn(len(handled))]
			log.Printf("User %s will follow model %s",
				user.Username, model.Username)
		} else {
			log.Printf("User %s will be themselves", user.Username)
		}
		event.Discussions.Iterate(func(disc *Discussion) error {
			r := rand.Intn(100)
			interest := 0

			// If we don't have a model, or feel like it (12.5%), do
			// our own thing; otherwise emulate our model.
			if model == nil || rand.Intn(8) == 0 {
				switch {
				case r >= 40:
					interest = rand.Intn(100)
				case r >= 50:
					interest = 100
				}
				log.Print(" Choosing own interest")
			} else {
				interest = model.Interest[disc.ID]
				log.Print(" Following mentor's interest")
			}

			log.Printf("Setting uid %s interest in discussion %s to %d",
				user.Username, disc.Title, interest)
			if err := user.SetInterestNosave(disc, interest); err != nil {
				log.Fatalf("Setting interest: %v", err)
			}
			return nil
		})
		handled = append(handled, user)
	}
	event.Save()
}

const (
	TestUsers = 8
	TestDisc  = 6
	TestSlots = 4
)

func TestPopulate() {
	panic("Not implemented")
	// event.Init(EventOptions{
	// 	AdminPassword: "xenroot"})
	//SetFlag(FlagTestMode, true)
	for i := 0; i < TestUsers; i++ {
		NewTestUser()
	}
	for i := 0; i < TestDisc; i++ {
		NewTestDiscussion(nil)
	}
}
