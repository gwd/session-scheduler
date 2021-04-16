package event

import (
	"math/rand"
	"runtime/debug"
	"testing"

	"github.com/icrowley/fake"
)

// Goal: To test transactions to make sure they're robust against failure / irrational results

// Transactions:

// - DeleteUser.  Interacts with event_interest (user and user's
//   discussions), event_discussions, and event_users.

// - NewDiscussion.  Interacts with event_discussions,
//   event_users.isverified, event_interest

// - DiscussionUpdate.  Interacts with event_discussions,
//   event_users.isverified, event_interest (if changing owner)

// - DeleteDiscussion.

func transactionRoutineUserCreateReadDelete(t *testing.T, iterations int, exitChan chan bool) {
	exit := true

	defer func() { exitChan <- exit }()

	for i := 0; i < iterations; i++ {
		// Make a user, then read them back
		user, res := testNewUser(t)
		if res {
			return
		}

		gotuser, err := UserFind(user.UserID)
		if err != nil {
			t.Errorf("ERROR: Finding the user we just created by ID: %v", err)
			return
		}
		if gotuser == nil {
			t.Errorf("ERROR: Couldn't find just-created user by id %s!", user.UserID)
			return
		}

		if !compareUsers(&user, gotuser, t) {
			t.Errorf("ERROR: User data mismatch")
			return
		}

		_, err = UserFindByUsername(user.Username)
		if err != nil {
			t.Errorf("ERROR: UserFindByUsername: %v", err)
			return
		}

		// Do some random user updates, make sure they "took"
		user.RealName = fake.FullName()
		user.Email = fake.EmailAddress()
		err = UserUpdate(&user, nil, "", "")
		if err != nil {
			t.Errorf("ERROR: Updating user: %v", err)
			return
		}

		gotuser, err = UserFind(user.UserID)
		if err != nil {
			t.Errorf("ERROR: Finding the user we just created by ID: %v", err)
			return
		}
		if gotuser == nil {
			t.Errorf("ERROR: Couldn't find just-created user by id %s!", user.UserID)
			return
		}

		if !compareUsers(&user, gotuser, t) {
			t.Errorf("ERROR: User data mismatch")
			return
		}

		_, err = gotuser.GetLocationTZ()
		if err != nil {
			t.Errorf("ERROR: Getting user's location: %v", err)
			return
		}

		for j := 0; j < 5; j++ {
			verified := rand.Intn(2) == 0
			err := user.SetVerified(verified)
			if err != nil {
				t.Errorf("ERROR: Changing verification: %v", err)
				return
			}
			user.IsVerified = verified
		}

		// Get all users; make sure we get at least one.
		users := []User{}
		err = UserIterate(func(u *User) error {
			users = append(users, *u)
			return nil
		})
		if err != nil {
			t.Errorf("ERROR: Getting list of all users: %v", err)
			return
		}
		if len(users) == 0 {
			t.Errorf("ERROR: UserIterate returned no users!")
			return
		}
		t.Logf("Found %d users total", len(users))

		users, err = UserGetAll()
		if err != nil {
			t.Errorf("ERROR: UserGetAll: %v", err)
			return
		}

		_, err = UserFindRandom()
		if err != nil {
			t.Errorf("ERROR: UserFindRandom: %v", err)
			return
		}

		// Make a bunch of discussions
		for j := 0; j < 5; j++ {
			disc, res := testNewDiscussion(t, user.UserID)
			if res {
				return
			}

			// Look for that discussion by did
			gotdisc, err := DiscussionFindByIdFull(disc.DiscussionID)
			if err != nil {
				t.Errorf("ERROR: Finding the discussion we just created by ID: %v", err)
				return
			}
			if gotdisc == nil {
				t.Errorf("ERROR: Couldn't find just-created discussion by id %s!", disc.DiscussionID)
				return
			}
			if !compareDiscussions(&disc, &gotdisc.Discussion, t) {
				t.Errorf("ERROR: Discussion data mismatch")
				return
			}
		}

		// Get all discussions & set an interest in some of them
		discussions := []Discussion{}
		err = DiscussionIterate(func(d *DiscussionFull) error {
			discussions = append(discussions, d.Discussion)
			return nil
		})
		if err != nil {
			t.Errorf("ERROR: Getting list of all open discussions: %v", err)
			return
		}
		t.Logf("Found %d discussions total", len(discussions))

		for j := 0; j < 20; j++ {
			didx := rand.Intn(len(discussions))
			interest := rand.Intn(100)
			if interest < 20 {
				interest = 0
			}
			err := user.SetInterest(&discussions[didx], interest)
			if err != nil && err != ErrUserOrDiscussionNotFound {
				t.Errorf("ERROR: Setting interest: %v", err)
				return
			}

			gotInterest, err := user.GetInterest(&discussions[didx])
			if err != nil {
				t.Errorf("ERROR user.GetInterest: %v", err)
				return
			}
			// A deleted discussion will return interest 0
			if gotInterest != interest && gotInterest != 0 {
				t.Errorf("ERROR: user.GetInterest: wanted %d, got %d!", interest, gotInterest)
				return
			}

			maxInt, err := discussions[didx].GetMaxScore()
			if err != nil {
				t.Errorf("ERROR discussions.MaxScore(): %v", err)
				return
			}
			if maxInt < interest && maxInt != 0 {
				t.Errorf("ERROR non-zero MaxScore %d < my own interest %d!", maxInt, interest)
				return
			}
		}

		// Get discussions for this user and perform some operations on them
		discussions = []Discussion{}
		err = DiscussionIterateUser(user.UserID, func(df *DiscussionFull) error {
			discussions = append(discussions, df.Discussion)
			return nil
		})
		if err != nil {
			t.Errorf("ERROR: Getting list of my own discussions: %v", err)
			return
		}

		// Update / SetPublic
		for j := 0; j < len(discussions)*2; j++ {
			var didx int
			for {
				didx = rand.Intn(len(discussions))
				if discussions[didx].DiscussionID != "" {
					break
				}
			}

			disc := &discussions[didx]

			public := rand.Intn(2) == 0
			err = DiscussionSetPublic(disc.DiscussionID, public)
			if err != nil {
				t.Errorf("ERROR: DiscussionSetPublic: %v", err)
				return
			}
			disc.IsPublic = public

			disc.Title = fake.Title()
			disc.Description = fake.Paragraphs()
			err = DiscussionUpdate(disc)
			if err != nil {
				t.Errorf("ERROR: DiscussionUpdate: %v", err)
				return
			}

			// NB we don't check values here because we don't want to have to deal w/ ApprovedTitle &c
		}

		for j := 0; j < len(discussions)/2; j++ {
			var didx int
			for {
				didx = rand.Intn(len(discussions))
				if discussions[didx].DiscussionID != "" {
					break
				}
			}

			if discussions[didx].Owner != user.UserID {
				t.Errorf("ERROR: User %v got discussion owned by %v!", user.UserID, discussions[didx].Owner)
				return
			}

			err = DeleteDiscussion(discussions[didx].DiscussionID)
			if err != nil {
				t.Errorf("ERROR: Deleting discussion %v owned by %v: %v", discussions[didx].DiscussionID, discussions[didx].Owner, err)
				return
			}
			discussions[didx].DiscussionID = ""
		}

		err = DeleteUser(user.UserID)
		if err != nil {
			t.Errorf("ERROR: Deleting user %s: %v", user.UserID, err)
			return
		}

		gotuser, err = UserFind(user.UserID)
		if err != nil {
			t.Errorf("ERROR: Error getting deleted user: %v", err)
			return
		}
		if gotuser != nil {
			t.Errorf("ERROR: Getting deleted user: Expected nil, got %v!", gotuser)
			return
		}

		err = DeleteUser(user.UserID)
		if err != ErrUserNotFound {
			t.Errorf("ERROR: Deleting non-existent user: wanted ErrUserNotfound, got %v", err)
			return
		}
	}

	exit = false
}

func testTransaction(t *testing.T) (exit bool) {
	// Any "early" exit is a failure
	exit = true

	tc := dataInit(t)
	if tc == nil {
		return
	}

	debug.SetTraceback("all")

	// Have several goroutines create and delete users
	{
		routineCount := 5
		iterationCount := 20
		exitChan := make(chan bool, routineCount)

		t.Logf("Testing UserCreateReadDelete (%d routines %d iterations)", routineCount, iterationCount)
		for i := 0; i < routineCount; i++ {
			go transactionRoutineUserCreateReadDelete(t, iterationCount, exitChan)
		}

		shouldExit := false
		for i := 0; i < routineCount; i++ {
			shouldExit = <-exitChan || shouldExit
		}

		close(exitChan)

		if shouldExit {
			return
		}
	}

	tc.cleanup()

	return false
}
