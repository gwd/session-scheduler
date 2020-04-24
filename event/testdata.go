package event

import (
	"log"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Try to emulate "realistic" interest, where people will be like one another.
// - Create four "unique" people at the beginning, with random interests
// - Afterwards, choose someone randomly to emulate 90% of the time.
// - When emulating somebody, choose like them 7/8 times
func TestGenerateInterest() {
	handled := []*User{}
	UserIterate(func(user *User) error {
		var model *User
		// Create 4 random "models" at first; after that, 10% are random
		if len(handled) > 4 && rand.Intn(10) != 0 {
			model = handled[rand.Intn(len(handled))]
			log.Printf("User %s will follow model %s",
				user.Username, model.Username)
		} else {
			log.Printf("User %s will be themselves", user.Username)
		}
		DiscussionIterate(func(disc *Discussion) error {
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
				// FIXME: Interest
				//interest = model.Interest[disc.ID]
				interest = 0
				log.Print(" Following mentor's interest")
			}

			log.Printf("Setting uid %s interest in discussion %s to %d",
				user.Username, disc.Title, interest)
			if err := user.SetInterest(disc, interest); err != nil {
				log.Fatalf("Setting interest: %v", err)
			}
			return nil
		})
		handled = append(handled, user)
		return nil
	})
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
		//NewTestUser()
	}
	for i := 0; i < TestDisc; i++ {
		//NewTestDiscussion(nil)
	}
}
