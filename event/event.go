package event

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"

	"github.com/gwd/session-scheduler/id"
)

var deepCopyBuffer bytes.Buffer
var deepCopyEncoder = gob.NewEncoder(&deepCopyBuffer)
var deepCopyDecoder = gob.NewDecoder(&deepCopyBuffer)

type EventStore struct {
	filename string
	*sqlx.DB

	Timetable Timetable

	Users       UserStore
	Discussions DiscussionStore
	Locations   LocationStore

	ScheduleV2    *Schedule
	ScheduleSlots int
	LockedSlots

	ScheduleState
}

type EventOptions struct {
	AdminPwd string
}

var event EventStore

const (
	StoreFilename = "data/event.json"
	DbFilename    = "data/event.sqlite"
	AdminUsername = "admin"
	// 3 days * 3 slots per day
	DefaultSlots = 9
)

func (store *EventStore) Init(adminPwd string) {
	store.Users.Init()
	store.Discussions.Init()
	store.Timetable.Init()
	store.ScheduleSlots = store.Timetable.GetSlots()

	store.Locations.Init()
	store.ScheduleV2 = nil

	store.LockedSlots = make([]bool, store.ScheduleSlots)

	if adminPwd == "" {
		adminPwd = id.GenerateRawID(12)
	}

	_, err := NewUser(adminPwd, User{Username: AdminUsername,
		IsAdmin:    true,
		IsVerified: true,
		RealName:   "Xen Schedule Administrator"})
	if err != nil {
		log.Fatalf("Error creating admin user: %v", err)
	}
	log.Printf("Administrator account: admin %s", adminPwd)

	event.Save()
}

// Reset "event" data, without touching users or discussions
func (store *EventStore) ResetEventData() {
	store.Timetable.Init()
	store.ScheduleSlots = store.Timetable.GetSlots()

	store.Locations.Init()
	store.ScheduleV2 = nil
	store.LockedSlots = make([]bool, store.ScheduleSlots)
	store.Discussions.ResetEventData()

	event.Save()
}

func ResetData() {
	event.ResetEventData()
}

// Reset "user" data -- users, discussions, and interest (keeping admin user).
// This also resets the 'event' data, as it won't make much sense anymore with the
// users and discussions gone.
// This should only be done in test mode!
func (store *EventStore) ResetUserData() {
	admin, err := store.Users.FindByUsername(AdminUsername)
	if err != nil || admin == nil {
		log.Fatalf("Can't find admin user: %v", err)
	}

	store.Users.Init()
	store.Discussions.Init()
	store.ResetEventData()

	event.Users.Save(admin)
}

func (store *EventStore) Load(opt EventOptions) error {
	if store.filename == "" {
		store.filename = StoreFilename
	}
	contents, err := ioutil.ReadFile(store.filename)

	if err != nil {
		if os.IsNotExist(err) {
			store.Init(opt.AdminPwd)
			return nil
		}
		return err
	}

	// Restoring from an existing file at this point
	err = json.Unmarshal(contents, store)
	if err != nil {
		return err
	}

	if opt.AdminPwd != "" {
		admin, err := store.Users.FindByUsername(AdminUsername)
		if err != nil || admin == nil {
			log.Fatalf("Can't find admin user: %v", err)
		}
		log.Printf("Resetting admin password")
		admin.SetPassword(opt.AdminPwd)
	}

	// Clean up any stale 'running' data
	if event.ScheduleState.IsRunning() {
		event.ScheduleState.SearchFailed()
	}

	// Run timetable placement to update discussion info
	if event.ScheduleV2 != nil {
		event.ScheduleV2.LoadPost()
		event.Timetable.Place(event.ScheduleV2)
	}
	return nil
}

func Load(opt EventOptions) error {
	return event.Load(opt)
}

func (store *EventStore) Save() error {
	contents, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(store.filename, contents, 0660)
}

type LockedSlots []bool

func (ls *LockedSlots) Set(new LockedSlots) {
	*ls = new
	event.Timetable.UpdateIsFinal(new)
	event.Save()
}

func LockedSlotsSet(new LockedSlots) {
	event.LockedSlots.Set(new)
}

type UserStore map[UserID]*User

func (ustore *UserStore) Init() {
	*ustore = UserStore(make(map[UserID]*User))
}

func (ustore UserStore) Save(user *User) error {
	ustore[user.ID] = user
	return event.Save()
}

func (ustore UserStore) Find(id UserID) (*User, error) {
	user, ok := ustore[id]
	if ok {
		return user, nil
	}
	return nil, nil
}

func UserFind(id UserID) (*User, error) {
	return event.Users.Find(id)
}

func (ustore UserStore) FindRandom() (user *User, err error) {
	// Don't count the admin user
	l := len(ustore) - 1

	if l == 0 {
		err = fmt.Errorf("No users!")
		return
	}

	// Choose a random value from [0,l) and
	i := rand.Int31n(int32(l))
	for _, user = range ustore {
		if user.Username == AdminUsername {
			continue
		}
		if i == 0 {
			return
		}
		i--
	}
	// We shouldn't be able to get here, but the compiler doesn't know that
	return
}

func (ustore UserStore) Iterate(f func(u *User) error) (err error) {
	for _, user := range ustore {
		err = f(user)
		if err != nil {
			return
		}
	}
	return
}

func UserIterate(f func(u *User) error) (err error) {
	return event.Users.Iterate(f)
}

func (ustore UserStore) GetUsers() (users []*User) {
	ustore.Iterate(func(u *User) error {
		if u.Username != AdminUsername {
			users = append(users, u)
		}
		return nil
	})

	sort.Slice(users, func(i, j int) bool {
		return users[i].ID < users[j].ID
	})

	return
}

func UserGetAll() (users []*User) {
	return event.Users.GetUsers()
}

func (ustore UserStore) FindByUsername(username string) (*User, error) {
	if username == "" {
		return nil, nil
	}

	for _, user := range ustore {
		if strings.ToLower(username) == strings.ToLower(user.Username) {
			return user, nil
		}
	}
	return nil, nil
}

func (ustore UserStore) FindByEmail(email string) (*User, error) {
	if email == "" {
		return nil, nil
	}

	for _, user := range ustore {
		if strings.ToLower(email) == strings.ToLower(user.Email) {
			return user, nil
		}
	}
	return nil, nil
}

func (ustore *UserStore) DeepCopy(ucopy *UserStore) (err error) {
	if err = deepCopyEncoder.Encode(ustore); err != nil {
		return err
	}
	if err = deepCopyDecoder.Decode(ucopy); err != nil {
		return err
	}
	return nil
}

func (ustore UserStore) Delete(uid UserID) error {
	delete(ustore, uid)
	event.ScheduleState.Modify()
	return event.Save()
}

func (dstore *DiscussionStore) DeepCopy(dcopy *DiscussionStore) (err error) {
	if err = deepCopyEncoder.Encode(dstore); err != nil {
		return err
	}
	if err = deepCopyDecoder.Decode(dcopy); err != nil {
		return err
	}
	return nil
}

type DiscussionStore map[DiscussionID]*Discussion

func (dstore *DiscussionStore) Init() {
	*dstore = DiscussionStore(make(map[DiscussionID]*Discussion))
}

// Update PossibleSlot size while retaining other information
func (dstore *DiscussionStore) ResetEventData() {
	dstore.Iterate(func(disc *Discussion) error {
		disc.PossibleSlots = MakePossibleSlots(event.ScheduleSlots)
		return nil
	})
}

func (dstore DiscussionStore) Find(id DiscussionID) (*Discussion, error) {
	discussion, exists := (dstore)[id]
	if !exists {
		return nil, nil
	}

	return discussion, nil
}

func (dstore DiscussionStore) Iterate(f func(d *Discussion) error) (err error) {
	for _, disc := range dstore {
		err = f(disc)
		if err != nil {
			return
		}
	}
	return
}

func DiscussionIterate(f func(d *Discussion) error) (err error) {
	return event.Discussions.Iterate(f)
}

func (dstore DiscussionStore) Save(discussion *Discussion) error {
	dstore[discussion.ID] = discussion
	return event.Save()
}

func (dstore DiscussionStore) Delete(did DiscussionID) error {
	delete(dstore, did)
	event.ScheduleState.Modify()
	return event.Save()
}

func (dstore DiscussionStore) GetDidListUser(uid UserID) (list []DiscussionID) {
	dstore.Iterate(func(d *Discussion) error {
		if d.Owner == uid {
			list = append(list, d.ID)
		}
		return nil
	})

	return
}

func ScheduleGetSlots() int {
	return event.ScheduleSlots
}
