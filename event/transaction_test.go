package event

import "testing"

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
		user, res := testNewUser(t)
		if res {
			return
		}

		gotuser, err := UserFind(user.UserID)
		if err != nil {
			t.Errorf("Finding the user we just created by ID: %v", err)
			return
		}
		if gotuser == nil {
			t.Errorf("Couldn't find just-created user by id %s!", user.UserID)
			return
		}

		if !compareUsers(&user, gotuser, t) {
			t.Errorf("User data mismatch")
			return
		}

		err = DeleteUser(user.UserID)
		if err != nil {
			t.Errorf("Deleting user %s: %v", user.UserID, err)
			return
		}

		gotuser, err = UserFind(user.UserID)
		if err != nil {
			t.Errorf("Error getting deleted user: %v", err)
			return
		}
		if gotuser != nil {
			t.Errorf("Getting deleted user: Expected nil, got %v!", gotuser)
			return
		}

		err = DeleteUser(user.UserID)
		if err != ErrUserNotFound {
			t.Errorf("Deleting non-existent user: wanted ErrUserNotfound, got %v", err)
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
			shouldExit = shouldExit || <-exitChan
		}

		close(exitChan)

		if shouldExit {
			return
		}
	}

	tc.cleanup()

	return false
}
