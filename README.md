session-scheduler is a tool for scheduling [Unconference-style](https://en.wikipedia.org/wiki/Unconference) discussion sections to maximize participation.  The basic idea is:

 1. The event creators define the "slots" available for sessions
 2. Members suggest sessions
 3. Attendees express how interested they are in attending various sessions
 4. The scheduler tries to find a schedule that maximizes utility

This is similar to Ubuntu's [Summit scheduler](https://launchpad.net/summit), but:

 1. Lighter-weight; doesn't require a launchpad account, anybody can create sessions, &c
 2. Allows users to express different levels of interest

At the moment, this is a prototype focused on Xen's Developer Summit.

# Build instructions

Make sure you have your `GOPATH` set up to something reasonable:

```
$ echo $GOPATH
/Users/dunlapg/go
```

Use `go get` to download the repo and all its dependencies:

```
go get -d github.com/gwd/session-scheduler
```

This will download the source and dependencies into your `$GOPATH`, but not compile it yet.

The session-scheduler must be run from the git repo; this will be in `$GOPATH/src/github.com/gwd/sessions-scheduler`.

Change to this directory:

```
cd $GOPATH/src/github.com/gwd/session-scheduler
```

Build the binary:

```
go build
```

And finally, make a directory for storing data:

```
mkdir data
```

# Starting instructions

Running it is simple.  From the git repo, run:

```
./session-scheduler
```

The first time you run `session-scheduler` it will create `data/event.json` to store data about the event.  It will also create an account named `admin`, generate a random password, and print the password to `stdout`.

It will then start serving http on localhost:3000.  To view the webpage, go to "http://localhost:3000".

# Admin mode

Logging in as `admin`, you are in "Admin mode".  The `admin` account may edit users and sessions, but not express interest in sessions.  If you want to experience life as a user, you'll have to create a user account.

The `admin` account has a "Console" page available.  From there you can initiate the session scheduler
and enable test mode, set the verification code, and other admin activities.

# Test mode

Test mode enables functionality useful for developing and testing the website and UI.  In particular it allows
you to:

1. Generate a random set of users, with random profiles
2. Generate random discussions (from the currently-present users)
3. Generate random "interest" values for current users in current discussions
4. Clear the user database (while keeping the current admin password).

There are under the admin console.

Because these are destructive, they are only available when "test mode" is enabled.

# Miscellaneous notes

Registration requires a verification code.  This will be generated randomly the first time session-scheduler
is run.  It can also be set from the admin console.

Discussion proposals refuse duplicate discussions.

Any given user is limited to generating sessions equal to the total number of session slots.
