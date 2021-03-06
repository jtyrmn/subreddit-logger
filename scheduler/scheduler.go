package scheduler

import (
	"errors"
	"fmt"
	"time"

	"github.com/jtyrmn/reddit-votewatch/reddit"
	"github.com/jtyrmn/reddit-votewatch/util"
)

//this file handles the timing and scheduling of certain events such as refreshing the access token, culling the db, requerying reddit, etc

type redditApiHandlerScheduler interface {
	TimeToNextTokenRefresh() time.Duration
	TokenRefresh() error

	TrackNewlyCreatedPosts() int
	GetTrackedPosts() reddit.ContentGroup

	GetTrackedIDs() []reddit.Fullname
	FetchPosts([]reddit.Fullname) (*reddit.ContentGroup, error)

	StopTrackingOldPosts(uint64) int
}

type databaseConnectionScheduler interface {
	RecordNewData(reddit.ContentGroup) error

	SaveListings(reddit.ContentGroup) error

	RecieveListings(reddit.ContentGroup, int64) (int, error)

	CullListings(uint64) (int, error)
}

//this function starts a forever loops that goes over all the events of both the reddit and database handler simultaneously
func Start(reddit redditApiHandlerScheduler, database databaseConnectionScheduler) {
	//before starting the loop, pull pre-existing listings from db
	pullFromDB(reddit, database)

	//ticker for reddit token refresh
	redditTicker := time.NewTicker(reddit.TimeToNextTokenRefresh())

	//ticker for fetching new posts
	newPostsTicker := time.NewTicker(time.Second * time.Duration(util.GetEnvInt("NEW_POSTS_REFRESH_PERIOD")))

	//ticker for downloading fetching new posts and downloading them to db
	updatePostsTicker := time.NewTicker(time.Second * time.Duration(util.GetEnvInt("UPDATE_TRACKED_POSTS_REFRESH_PERIOD")))

	//ticker for untracking posts that are past a certain age
	untrackPostsTicker := time.NewTicker(time.Second * time.Duration(util.GetEnvInt("UNTRACK_POSTS_REFRESH_PERIOD")))

	//ticker for culling old posts
	cullPostsTicker := time.NewTicker(time.Second * time.Duration(util.GetEnvInt("CULL_POSTS_REFRESH_PERIOD")))


	logOutput("starting scheduler\n")
	for {
		select {
		case <-redditTicker.C:
			refreshToken(reddit, *redditTicker)

		case <-newPostsTicker.C:
			fetchNewPosts(reddit, database)

		case <-updatePostsTicker.C:
			err := updateTrackedPosts(reddit, database)
			if err != nil {
				logOutputError("error updating:\n" + err.Error())
			}

		case <-untrackPostsTicker.C:
			stopTrackingOldPosts(reddit)

		case <-cullPostsTicker.C:
			cullDatabase(database)
		}
		fmt.Println() //create spacing between the different events
	}
}

//following functions are just wrappers for self-explanatory behaviour

func pullFromDB(reddit redditApiHandlerScheduler, database databaseConnectionScheduler) {
	logOutput("pulling from db...")

	maxAge := util.GetEnvInt("MAX_TRACKING_AGE")

	insertions, err := database.RecieveListings(reddit.GetTrackedPosts(), int64(maxAge)) //reddit API handler's tracked posts <<< posts from db
	if err != nil {
		logOutputError("warning: error recieving listings from database:\n" + err.Error())
	}
	logOutput(fmt.Sprintf("%d posts recieved from database\n", insertions))
}

func refreshToken(reddit redditApiHandlerScheduler, redditTicker time.Ticker) {
	logOutput("refreshing access token...")
	err := reddit.TokenRefresh()
	if err != nil {
		logOutputError("error refreshing access token:\n" + err.Error())
	}
	redditTicker.Reset(reddit.TimeToNextTokenRefresh())
}

func fetchNewPosts(reddit redditApiHandlerScheduler, database databaseConnectionScheduler) {
	logOutput("fetching new posts...")
	count := reddit.TrackNewlyCreatedPosts()
	logOutput(fmt.Sprintf("%d new posts tracked", count))
	logOutput(fmt.Sprintf("%d total posts tracked", len(reddit.GetTrackedPosts())))

	if count == 0 { //no need to save new posts if there are no new posts
		return
	}
	
	logOutput("saving posts...")
	err := database.SaveListings(reddit.GetTrackedPosts())
	if err != nil {
		logOutputError("error saving posts:\n" + err.Error())
	}
}

func updateTrackedPosts(reddit redditApiHandlerScheduler, database databaseConnectionScheduler) error {
	logOutput("updating posts...")

	IDs := reddit.GetTrackedIDs()

	posts, err := reddit.FetchPosts(IDs)
	if err != nil {
		return errors.New("error fetching posts from reddit:\n" + err.Error())
	}

	err = database.RecordNewData(*posts)
	if err != nil {
		return errors.New("error recording data in database:\n" + err.Error())
	}

	return nil
}

func stopTrackingOldPosts(reddit redditApiHandlerScheduler) {
	untrackedPosts := reddit.StopTrackingOldPosts(uint64(util.GetEnvInt("MAX_TRACKING_AGE")))
	if untrackedPosts > 0 {
		logOutput(fmt.Sprintf("no longer tracking %d old posts", untrackedPosts))
	}
}

func cullDatabase(database databaseConnectionScheduler) {
	logOutput("culling posts...")

	deletedPosts, err := database.CullListings(uint64(util.GetEnvInt("CULLING_AGE")))
	if err != nil {
		logOutputError("error culling database:\n" + err.Error())
		return
	}

	logOutput(fmt.Sprintf("culled %d posts", deletedPosts))
}

//pretty formatted printing
func logOutput(str string) {
	fmt.Printf("\033[0;36m%s\033[0m: %s\n", time.Now().Format(time.ANSIC), str)
}

func logOutputError(str string) {
	fmt.Printf("\033[0;36m%s\033[0m: \033[0;31m%s\033[0m\n", time.Now().Format(time.ANSIC), str)
}
