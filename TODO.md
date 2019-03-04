# Short-term usability improvements

* Document `deploy.sh` and `run.sh` expected use case
* Document inactive mode, &c

# Priority improvements

* Allow enabling / disabling schedule
* Deal with account creation restriction
 - Have session 'moderation' for 'unverified' accounts
* Having a place to take notes during the session
    * Integration of etherpad-like functionality?
    * Automatically creating an etherpad (with a link)?
  	    * `etherpad.net/p/$UUID`
	    * `UUID` could be the session uuid, or a new one (since session uuids are pseudo-public)
    * hackmd.io?
* Avoid scheduling multiple sessions proposed by the same person at the same time

# Potential improvements

* Config file for setting schedule rather than hard-coding
* Creating / editing schedule in webapp
* Admin: Allow admin to delete users
* Admin: Allow to delete all sessions from a user
* Make webserver actually multi-threaded (get rid of Big Lock)
* Automated backup

# Clean-up

* Lots of visual improvements
* Make structure more rational
