package event

import (
	"math/rand"
	"testing"
)

func checkInterest(interestMap [][]int, users []User, discussions []Discussion, t *testing.T) bool {
	eMaxScore := make([]int, len(discussions))
	for uidx := range users {
		for didx := range discussions {
			interest, err := users[uidx].GetInterest(&discussions[didx])
			if err != nil {
				t.Errorf("[%d][%d] GetInterest returned %v", uidx, didx, err)
				return true
			}
			if interest != interestMap[uidx][didx] {
				t.Errorf("[%d][%d] expected %d got %d", uidx, didx, interestMap[uidx][didx], interest)
				return true
			}
			eMaxScore[didx] += interest
		}
	}

	for didx := range discussions {
		maxScore, err := discussions[didx].GetMaxScore()
		if err != nil {
			t.Errorf("ERROR GetMaxScore: %v", err)
			return true
		}
		if maxScore != eMaxScore[didx] {
			t.Errorf("maxscore[%d]: expected %d, got %d!", didx, eMaxScore[didx], maxScore)
			return true
		}
	}
	return false
}

func testUnitInterest(t *testing.T) (exit bool) {
	exit = true

	tc := dataInit(t)
	if tc == nil {
		return
	}

	// Make 6 users for testing
	testUserCount := 6
	users := make([]User, testUserCount)
	userMap := make(map[UserID]int)

	// Enough discussions to max out one user, plus a few for others
	testDiscussionCount := testUserCount * 2
	discussions := make([]Discussion, testDiscussionCount)

	interestMap := make([][]int, testUserCount)

	for i := range users {
		subexit := false
		users[i], subexit = testNewUser(t)
		if subexit {
			return
		}
		userMap[users[i].UserID] = i
		interestMap[i] = make([]int, testDiscussionCount)
	}

	t.Logf("Making some discussions")
	for i := range discussions {
		subexit := false
		discussions[i], subexit = testNewDiscussion(t, "")
		if subexit {
			return
		}

		owneridx, prs := userMap[discussions[i].Owner]
		if !prs {
			t.Errorf("Unknown user %v!", discussions[i].Owner)
			return
		}
		owner := &users[owneridx] // Wish I had 'const'

		interest, err := owner.GetInterest(&discussions[i])
		if err != nil {
			t.Errorf("ERROR GetInterest: %v", err)
			return
		}
		if interest != InterestMax {
			t.Errorf("Unexpected interest for new discussion: got %d expected %d",
				interest, InterestMax)
			return
		}

		interestMap[owneridx][i] = InterestMax
	}

	// Tests users that haven't expressed an interest yet
	t.Logf("Checking initial interest values")
	if checkInterest(interestMap, users, discussions, t) {
		return
	}

	t.Logf("Setting invalid values")
	// Test setting invalid values
	if err := users[0].SetInterest(&discussions[0], -1); err != errInvalidInterest {
		t.Errorf("Unexpected result in setting interest to -1: %v", err)
		return
	}
	if err := users[0].SetInterest(&discussions[0], InterestMax+1); err != errInvalidInterest {
		t.Errorf("Unexpected result in setting interest to InterestMax+1: %v", err)
		return
	}

	// Test with bogus users and discussions
	t.Logf("Testing with bogus users and discussions")
	{
		user := User{UserID: UserID("NotAUser")}
		err := user.SetInterest(&discussions[0], 50)
		if err == nil {
			t.Errorf("Unexpectedly succeeded setting interest for an invalid user!")
			return
		}

		discussion := &Discussion{DiscussionID: DiscussionID("NotADiscussion")}
		err = users[0].SetInterest(discussion, 50)
		if err == nil {
			t.Errorf("Unexpectedly succeeded setting interest for an invalid discussion!")
			return
		}
	}

	// Test setting valid non-zero values
	//  - On previously zero values
	t.Logf("Moving 0 -> nonzero")
	for count := 0; count < len(discussions)*2; {
		uidx := rand.Int31n(int32(len(users)))
		didx := rand.Int31n(int32(len(discussions)))

		if interestMap[uidx][didx] > 0 {
			continue
		}

		interest := rand.Intn(InterestMax-1) + 1
		t.Logf(" [%d][%d] %d -> %d", uidx, didx, interestMap[uidx][didx], interest)
		if err := users[uidx].SetInterest(&discussions[didx], interest); err != nil {
			t.Errorf("Trying to set interest [%d][%d] to %d: %v",
				uidx, didx, interest, err)
			return
		}
		interestMap[uidx][didx] = interest

		count++
	}

	if checkInterest(interestMap, users, discussions, t) {
		return
	}

	//  - On previously non-zero values
	t.Logf("Moving nonzero -> nonzero")
	for count := 0; count < len(discussions)*2; {
		uidx := rand.Int31n(int32(len(users)))
		didx := rand.Int31n(int32(len(discussions)))

		if interestMap[uidx][didx] == 0 {
			continue
		}

		interest := rand.Intn(InterestMax-1) + 1
		t.Logf(" [%d][%d] %d -> %d", uidx, didx, interestMap[uidx][didx], interest)
		if err := users[uidx].SetInterest(&discussions[didx], interest); err != nil {
			t.Errorf("Trying to set interest [%d][%d] to %d: %v",
				uidx, didx, interest, err)
			return
		}
		interestMap[uidx][didx] = interest

		count++
	}

	if checkInterest(interestMap, users, discussions, t) {
		return
	}

	// Test setting zero values
	//  - On previously non-zero values
	t.Logf("Moving nonzero -> zero")
	for count := 0; count < len(discussions)*2; {
		uidx := rand.Int31n(int32(len(users)))
		didx := rand.Int31n(int32(len(discussions)))

		if interestMap[uidx][didx] == 0 {
			continue
		}

		interest := 0
		t.Logf(" [%d][%d] %d -> %d", uidx, didx, interestMap[uidx][didx], interest)
		if err := users[uidx].SetInterest(&discussions[didx], interest); err != nil {
			t.Errorf("Trying to set interest [%d][%d] to %d: %v",
				uidx, didx, interest, err)
			return
		}
		interestMap[uidx][didx] = interest

		count++
	}

	if checkInterest(interestMap, users, discussions, t) {
		return
	}

	//  - On previously zero values
	t.Logf("Moving zero -> zero")
	for count := 0; count < len(discussions)*2; {
		uidx := rand.Int31n(int32(len(users)))
		didx := rand.Int31n(int32(len(discussions)))

		if interestMap[uidx][didx] > 0 {
			continue
		}

		interest := 0
		t.Logf(" [%d][%d] %d -> %d", uidx, didx, interestMap[uidx][didx], interest)
		if err := users[uidx].SetInterest(&discussions[didx], interest); err != nil {
			t.Errorf("Trying to set interest [%d][%d] to %d: %v",
				uidx, didx, interest, err)
			return
		}
		interestMap[uidx][didx] = interest

		count++
	}

	if checkInterest(interestMap, users, discussions, t) {
		return
	}

	// Test changing owner; interest should be 100
	t.Logf("Changing ownership")
	for count := 0; count < len(discussions)/4+1; count++ {
		var uidx, didx int32
		for {
			uidx = rand.Int31n(int32(len(users)))
			didx = rand.Int31n(int32(len(discussions)))

			if discussions[didx].Owner != users[uidx].UserID {
				break
			}
		}

		copy := discussions[didx]
		copy.Owner = users[uidx].UserID
		err := DiscussionUpdate(&copy)
		if err != nil {
			t.Errorf("Changing discussion owner: %v", err)
			return
		}
		interestMap[uidx][didx] = InterestMax
	}

	if checkInterest(interestMap, users, discussions, t) {
		return
	}

	// Test a discussion that's been deleted?
	t.Logf("Testing deleted discussions")
	{
		didx := len(discussions) - 1
		if didx < 0 {
			t.Errorf("Not enough discussions!")
			return
		}

		// First set the interest for user 0 to non-zero
		users[0].SetInterest(&discussions[didx], InterestMax)

		// Then delete the discussion
		err := DeleteDiscussion(discussions[didx].DiscussionID)
		if err != nil {
			t.Errorf("Deleting discussion: %v", err)
			return
		}

		// Then get the interest and make sure it's zero
		interest, err := users[0].GetInterest(&discussions[didx])
		if err != nil {
			t.Errorf("ERROR GetInterest: %v", err)
			return
		}
		if interest != 0 {
			t.Errorf("Expected 0, got %d for deleted discussion!", interest)
			return
		}

		discussions = discussions[:len(discussions)-1]

		for i := range interestMap {
			// NB discussions has already been shortened, so this shoul DTRT
			interestMap[i] = interestMap[i][:len(discussions)]
		}
	}

	if checkInterest(interestMap, users, discussions, t) {
		return
	}

	// Test a user that's been deleted?
	t.Logf("Testing deleted users")
	{
		// First, assign all of user N's discussions to user 0.
		uidx := len(users) - 1
		toDelete := []Discussion{}
		err := DiscussionIterateUser(users[uidx].UserID, func(d *Discussion) error {
			toDelete = append(toDelete, *d)
			return nil
		})
		for i := range toDelete {
			toDelete[i].Owner = users[0].UserID
			err = DiscussionUpdate(&toDelete[i])
			if err != nil {
				t.Errorf("Changing discussion owner: %v", err)
				return
			}
		}

		// NB: interestMap is now invalid unless we go through and
		// update users[0]'s interest in the re-homed discussions,
		// which requires reverse-mapping discussionid to didx.
		interestMap = nil

		// Set user N's interest in at least one discussion to non-zero
		err = users[uidx].SetInterest(&discussions[0], InterestMax)
		if err != nil {
			t.Errorf("Setting interest: %v", err)
			return
		}

		// Delete the user
		err = DeleteUser(users[uidx].UserID)
		if err != nil {
			t.Errorf("Deleting user: %v", err)
			return
		}

		// Check to see that the interest is zero
		interest, err := users[uidx].GetInterest(&discussions[0])
		if err != nil {
			t.Errorf("ERROR: GetInterest %v", err)
			return
		}
		if interest != 0 {
			t.Errorf("Expected interest 0 from deleted user, got %d!", interest)
			return
		}
		users = users[:uidx]
	}

	tc.cleanup()

	return false
}
