package event

import (
	"fmt"
	"sort"
	"testing"

	"github.com/icrowley/fake"
)

// This password should always be suitable
const TestPassword = "xenuser"

var lastIsVerified bool

var TestAlternateLocation = "America/Detroit"

func testNewUser(t *testing.T) (User, bool) {
	user := User{
		Username:    fake.UserName(),
		RealName:    fake.FullName(),
		Company:     fake.Company(),
		Email:       fake.EmailAddress(),
		Description: fake.Paragraphs(),
	}

	user.IsVerified = !lastIsVerified
	lastIsVerified = user.IsVerified

	var err error

	if user.IsVerified {
		user.Location, err = LoadLocation(TestAlternateLocation)
		if err != nil {
			t.Errorf("ERROR: LoadLocation(%s): %v", TestAlternateLocation, err)
			return user, true
		}
	}

	for _, err = NewUser(TestPassword, &user); err != nil; _, err = NewUser(TestPassword, &user) {
		// Just keep trying random usernames until we get a new one
		if err == errUsernameExists {
			user.Username = fake.UserName()
			t.Logf(" User exists!  Trying username %s instead", user.Username)
			continue
		}
		t.Errorf("Creating a test user: %v", err)
		return user, true
	}

	return user, false
}

func compareUsers(u1, u2 *User, t *testing.T) bool {
	ret := true
	if u1.UserID != u2.UserID {
		t.Logf("mismatch UserID: %v != %v", u1.UserID, u2.UserID)
		ret = false
	}
	if u1.Username != u2.Username {
		t.Logf("mismatch Username: %v != %v", u1.Username, u2.Username)
		ret = false
	}
	if u1.IsAdmin != u2.IsAdmin {
		t.Logf("mismatch IsAdmin: %v != %v", u1.IsAdmin, u2.IsAdmin)
		ret = false
	}
	if u1.IsVerified != u2.IsVerified {
		t.Logf("mismatch IsVerified: %v != %v", u1.IsVerified, u2.IsVerified)
		ret = false
	}
	if u1.RealName != u2.RealName {
		t.Logf("mismatch RealName: %v != %v", u1.RealName, u2.RealName)
		ret = false
	}
	if u1.Email != u2.Email {
		t.Logf("mismatch Email: %v != %v", u1.Email, u2.Email)
		ret = false
	}
	if u1.Company != u2.Company {
		t.Logf("mismatch Company: %v != %v", u1.Company, u2.Company)
		ret = false
	}
	if u1.Description != u2.Description {
		t.Logf("mismatch Description: %v != %v", u1.Description, u2.Description)
		ret = false
	}
	if u1.Location.String() != u2.Location.String() {
		t.Logf("mismatch Location: %v != %v", u1.Location, u2.Location)
		ret = false
	}
	return ret
}

// If true, test should be aborted
func testUnitUser(t *testing.T) (exit bool) {
	// Any "early" exit is a failure
	exit = true

	tc := dataInit(t)
	if tc == nil {
		return
	}

	t.Logf("Testing UserFindRandom when there are no users")
	{
		gotuser, err := UserFindRandom()
		if err != nil {
			t.Errorf("ERROR: UserFindRandom: %v", err)
			return
		}
		if gotuser != nil {
			t.Errorf("ERROR: UserFindRandom: Expected nil, got %v!", gotuser)
			return
		}
	}

	t.Logf("Testing admin password reset")
	{
		gotuser, err := UserFindByUsername(AdminUsername)
		if err != nil {
			t.Errorf("Finding the user we just created by username: %v", err)
			return
		}

		err = gotuser.setPassword(TestPassword)
		if err != nil {
			t.Errorf("ERROR: Setting admin password: %v", err)
			return
		}

		if !gotuser.CheckPassword(TestPassword) {
			t.Errorf("Admin password reset failed!")
			return
		}
	}

	// Make 5 users, and use them to unit-test various functions
	testUserCount := 5
	users := make([]User, testUserCount)

	for i := range users {
		subexit := false
		users[i], subexit = testNewUser(t)
		if subexit {
			return
		}

		// Try creating a new user with the same userid
		userCopy := users[i]
		_, err := NewUser(TestPassword, &userCopy)
		if err != errUsernameExists {
			t.Errorf("Testing duplicate username: expected errUsernameExists, got %v!", err)
			return
		}

		// Look for that user by uid
		gotuser, err := UserFind(users[i].UserID)
		if err != nil {
			t.Errorf("Finding the user we just created by ID: %v", err)
			return
		}
		if gotuser == nil {
			t.Errorf("Couldn't find just-created user by id %s!", users[i].UserID)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch")
			return
		}

		if !gotuser.CheckPassword(TestPassword) {
			t.Errorf("Password failed")
			return
		}

		// Look for that user by username
		gotuser, err = UserFindByUsername(users[i].Username)
		if err != nil {
			t.Errorf("Finding the user we just created by username: %v", err)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch")
			return
		}

		// Look for that user by username, checking password
		gotuser, err = UserFindByUsername(users[i].Username)
		if err != nil {
			t.Errorf("Finding the user we just created by username: %v", err)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch")
			return
		}

		gotuser, err = UserFindRandom()
		if err != nil {
			t.Errorf("Finding a random user: %v", err)
			return
		}

		// If this is our first user, there's only one possibility
		if i == 0 && !compareUsers(&users[i], gotuser, t) {
			t.Errorf("Random user didn't return the only non-admin user!")
			return
		}

		if gotuser.Username == AdminUsername {
			t.Errorf("Random user returned admin!")
			return
		}
	}

	{
		t.Logf("Testing searching for non-existing UserID")
		var userid UserID
		userid.generate()
		gotuser, err := UserFind(userid)
		if err != nil {
			t.Errorf("UserFind on non-existend userid: %v", err)
			return
		}
		if gotuser != nil {
			t.Errorf("UserFind on non-existent userid: Wanted nil, got %v!", gotuser)
			return
		}

		username := fake.UserName()
		// FIXME: Maybe check to see if the username doesn't exist already
		gotuser, err = UserFindByUsername(username)
		if err != nil {
			t.Errorf("UserFindByUsername on non-existend username %s: %v", username, err)
			return
		}
		if gotuser != nil {
			t.Errorf("UserFindByUsername on non-existent username %s: Wanted nil, got %v!", username, gotuser)
			return
		}
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].UserID < users[j].UserID
	})

	t.Logf("Testing UserGetAll")
	{
		gotusers, err := UserGetAll()
		if err != nil {
			t.Errorf("Getting a list of all users: %v", err)
			return
		}

		// Number of users should be number of test users + admin account
		if len(gotusers) != testUserCount+1 {
			t.Errorf("Expected %d users, got %d!", testUserCount+1, len(gotusers))
			return
		}

		// gotusers should be listed in UserID order.  Compare the two
		// slices, skipping over the admin user.
		i, j := 0, 0
		for i < testUserCount {
			if gotusers[j].Username == AdminUsername {
				j++
				continue
			}
			if !compareUsers(&users[i], &gotusers[j], t) {
				t.Errorf("UserGetAll mismatch")
				return
			}
			i++
			j++

		}
	}

	t.Logf("Testing UserIterate")
	{
		i := 0
		err := UserIterate(func(u *User) error {
			if u.Username == AdminUsername {
				return nil
			}
			if !compareUsers(&users[i], u, t) {
				return fmt.Errorf("UserIterate mismatch")
			}
			i++
			return nil
		})
		if err != nil {
			t.Errorf("UserIterate error: %v", err)
			return
		}
		if i != len(users) {
			t.Errorf("UserIterate: expected %d function calls, got %d!", len(users), i)
		}
	}

	t.Logf("Testing UserIterate error reporting")
	{
		i := 0
		err := UserIterate(func(u *User) error {
			if u.Username == AdminUsername {
				return ErrInternal
			}
			if !compareUsers(&users[i], u, t) {
				return fmt.Errorf("UserIterate mismatch")
			}
			i++
			return nil
		})
		if err != ErrInternal {
			t.Errorf("UserIterate error handling: wanted %v, got %v", ErrInternal, err)
			return
		}
	}

	// UserUpdate
	// - Passwords
	//  - "Normal" changing password
	//  - Changing with wrong password
	// - Changing username in field shouldn't actually change username
	// - Invalid userid
	t.Logf("Testing UserUpdate")
	for i := range users {
		// Change some things but not everything
		users[i].RealName = fake.FullName()
		users[i].Description = fake.Paragraphs()
		// Don't update password
		err := UserUpdate(&users[i], nil, "", "")
		if err != nil {
			t.Errorf("Updating user: %v", err)
			return
		}

		// Check to see that we still get the same result
		gotuser, err := UserFindByUsername(users[i].Username)
		if err != nil {
			t.Errorf("Finding the user we just created by username: %v", err)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch for user uid %s username %s",
				users[i].UserID, users[i].Username)
			return
		}

		if !gotuser.CheckPassword(TestPassword) {
			t.Errorf("Password failed")
			return
		}

		// Try changing some different things
		users[i].Company = fake.Company()
		if users[i].Location.String() == TestAlternateLocation {
			users[i].Location, err = LoadLocation(TestDefaultLocation)
		} else {
			users[i].Location, err = LoadLocation(TestAlternateLocation)
		}
		if err != nil {
			t.Errorf("Loading location: %v", err)
			return
		}
		// Don't update password
		err = UserUpdate(&users[i], nil, "", "")
		if err != nil {
			t.Errorf("Updating user: %v", err)
			return
		}

		// Check to see that we still get the same result
		gotuser, err = UserFindByUsername(users[i].Username)
		if err != nil {
			t.Errorf("Finding the user we just created by username: %v", err)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch for user uid %s username %s",
				users[i].UserID, users[i].Username)
			return
		}

		if !gotuser.CheckPassword(TestPassword) {
			t.Errorf("Password failed")
			return
		}

		// Try changing the password
		pwd2 := TestPassword + "2"

		err = UserUpdate(&users[i], &users[i], TestPassword, pwd2)
		if err != nil {
			t.Errorf("Changing password: %v", err)
			return
		}

		// Check that the change worked
		gotuser, err = UserFindByUsername(users[i].Username)
		if err != nil {
			t.Errorf("Finding the user we just created by username: %v", err)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch for user uid %s username %s",
				users[i].UserID, users[i].Username)
			return
		}

		if !gotuser.CheckPassword(pwd2) {
			t.Errorf("New password failed")
			return
		}

		// Try using the wrong password
		err = UserUpdate(&users[i], &users[i], TestPassword, pwd2)
		if err == nil {
			t.Errorf("Expected wrong password to fail, but succeeded!")
			return
		}

		// Change it back
		err = UserUpdate(&users[i], &users[i], pwd2, TestPassword)
		if err != nil {
			t.Errorf("Changing password back: %v", err)
			return
		}

		gotuser, err = UserFindByUsername(users[i].Username)
		if err != nil {
			t.Errorf("Finding the user we just created by username: %v", err)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch for user uid %s username %s",
				users[i].UserID, users[i].Username)
			return
		}

		if !gotuser.CheckPassword(TestPassword) {
			t.Errorf("Re-set password failed")
			return
		}

		// Try changing the username, and make sure it didn't actually change
		copy := users[i]
		copy.Username = "invalid"
		err = UserUpdate(&copy, nil, "", "")
		if err != nil {
			t.Errorf("Updating user: %v", err)
			return
		}

		gotuser, err = UserFindByUsername(users[i].Username)
		if err != nil {
			t.Errorf("Finding the attempted-to-change username old username: %v", err)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch for user uid %s username %s",
				users[i].UserID, users[i].Username)
			return
		}

		// Try changing IsAdmin and IsVerified
		copy = users[i]
		copy.IsAdmin = !copy.IsAdmin
		copy.IsVerified = !copy.IsVerified
		err = UserUpdate(&copy, nil, "", "")
		if err != nil {
			t.Errorf("Updating user: %v", err)
			return
		}

		gotuser, err = UserFindByUsername(users[i].Username)
		if err != nil {
			t.Errorf("Finding fake-modified user: %v", err)
			return
		}

		if !compareUsers(&users[i], gotuser, t) {
			t.Errorf("User data mismatch for user uid %s username %s",
				users[i].UserID, users[i].Username)
			return
		}

		// Try using a bogus UserID and make sure it fails
		copy.UserID = UserID("invalid")
		err = UserUpdate(&copy, nil, "", "")
		if err == nil {
			t.Errorf("Updating user with bad UserID didn't fail!")
			return
		}

		// Try modifying HashedPassword directly and make sure nothing changes
		copy = users[i]
		copy.HashedPassword, _ = passwordHash("Foo")
		err = UserUpdate(&copy, nil, "", "")
		if err != nil {
			t.Errorf("Updating user: %v", err)
			return
		}

		gotuser, err = UserFindByUsername(users[i].Username)
		if err != nil {
			t.Errorf("Finding fake-modified user: %v", err)
			return
		}

		if !gotuser.CheckPassword(TestPassword) {
			t.Errorf("Fake-modifying password worked!")
			return
		}

		if gotuser.CheckPassword("") {
			t.Errorf("Empty password works!")
			return
		}
	}

	// DeleteUser
	t.Logf("Testing DeleteUser")
	for i := range users {
		err := DeleteUser(users[i].UserID)
		if err != nil {
			t.Errorf("Deleting user %s: %v", users[i].UserID, err)
			return
		}

		gotuser, err := UserFind(users[i].UserID)
		if err != nil {
			t.Errorf("Error getting deleted user: %v", err)
			return
		}
		if gotuser != nil {
			t.Errorf("Getting deleted user: Expected nil, got %v!", gotuser)
			return
		}

		err = DeleteUser(users[i].UserID)
		if err != ErrUserNotFound {
			t.Errorf("Deleting non-existent user: wanted ErrUserNotfound, got %v", err)
			return
		}
	}

	tc.cleanup()

	return false
}
