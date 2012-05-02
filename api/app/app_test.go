package app

import (
	"fmt"
	"github.com/timeredbull/tsuru/api/auth"
	"github.com/timeredbull/tsuru/db"
	. "launchpad.net/gocheck"
	"launchpad.net/mgo"
	"launchpad.net/mgo/bson"
	"os"
	"path"
	"testing"
)

type hasAccessToChecker struct{}

func (c *hasAccessToChecker) Info() *CheckerInfo {
	return &CheckerInfo{Name: "HasAccessTo", Params: []string{"team", "app"}}
}

func (c *hasAccessToChecker) Check(params []interface{}, names []string) (bool, string) {
	if len(params) != 2 {
		return false, "you must provide two parameters"
	}
	team, ok := params[0].(auth.Team)
	if !ok {
		return false, "first parameter should be a team instance"
	}
	app, ok := params[1].(App)
	if !ok {
		return false, "second parameter should be an app instance"
	}
	return app.hasTeam(&team), ""
}

var HasAccessTo Checker = &hasAccessToChecker{}

func Test(t *testing.T) { TestingT(t) }

type S struct {
	session *mgo.Session
	team    auth.Team
}

var _ = Suite(&S{})

func (s *S) SetUpSuite(c *C) {
	var err error
	db.Session, err = db.Open("127.0.0.1:27017", "tsuru_app_test")
	c.Assert(err, IsNil)
	s.team = auth.Team{Name: "tsuruteam"}
	db.Session.Teams().Insert(s.team)
}

func (s *S) TearDownSuite(c *C) {
	defer db.Session.Close()
	db.Session.DropDB()
}

func (s *S) TearDownTest(c *C) {
	var apps []App
	err := db.Session.Apps().Find(nil).All(&apps)
	c.Assert(err, IsNil)
	for _, app := range apps {
		err = app.Destroy()
		c.Assert(err, IsNil)
	}
}

func (s *S) TestNewRepository(c *C) {
	a := App{Name: "foobar"}
	err := NewRepository(&a)
	c.Assert(err, IsNil)

	repoPath := GetRepositoryPath(&a)
	_, err = os.Open(repoPath) // test if repository dir exists
	c.Assert(err, IsNil)

	_, err = os.Open(path.Join(repoPath, "config"))
	c.Assert(err, IsNil)

	err = os.RemoveAll(repoPath)
	c.Assert(err, IsNil)
}

func (s *S) TestDeleteGitRepository(c *C) {
	a := &App{Name: "someApp"}
	repoPath := GetRepositoryPath(a)

	err := NewRepository(a)
	c.Assert(err, IsNil)

	_, err = os.Open(path.Join(repoPath, "config"))
	c.Assert(err, IsNil)

	DeleteRepository(a)
	_, err = os.Open(repoPath)
	c.Assert(err, NotNil)
}

func (s *S) TestGetRepositoryUrl(c *C) {
	a := App{Name: "foobar"}
	url := GetRepositoryUrl(&a)
	expected := fmt.Sprintf("git@tsuru.plataformas.glb.com:%s.git", a.Name)
	c.Assert(url, Equals, expected)
}

func (s *S) TestGetRepositoryName(c *C) {
	a := App{Name: "someApp"}
	obtained := GetRepositoryName(&a)
	expected := fmt.Sprintf("%s.git", a.Name)
	c.Assert(obtained, Equals, expected)
}

func (s *S) TestGetRepositoryPath(c *C) {
	a := App{Name: "someApp"}
	home := os.Getenv("HOME")
	obtained := GetRepositoryPath(&a)
	expected := path.Join(home, "../git", GetRepositoryName(&a))
	c.Assert(obtained, Equals, expected)
}

func (s *S) TestAll(c *C) {
	expected := make([]App, 0)
	app1 := App{Name: "app1", Teams: []auth.Team{}}
	app1.Create()
	expected = append(expected, app1)
	app2 := App{Name: "app2", Teams: []auth.Team{}}
	app2.Create()
	expected = append(expected, app2)
	app3 := App{Name: "app3", Teams: []auth.Team{}}
	app3.Create()
	expected = append(expected, app3)

	appList, err := AllApps()
	c.Assert(err, IsNil)
	c.Assert(expected, DeepEquals, appList)

	app1.Destroy()
	app2.Destroy()
	app3.Destroy()
}

func (s *S) TestGet(c *C) {
	newApp := App{Name: "myApp", Framework: "django", Teams: []auth.Team{}}
	err := newApp.Create()
	c.Assert(err, IsNil)

	myApp := App{Name: "myApp"}
	err = myApp.Get()
	c.Assert(err, IsNil)
	c.Assert(myApp, DeepEquals, newApp)

	err = myApp.Destroy()
	c.Assert(err, IsNil)
}

func (s *S) TestDestroy(c *C) {
	a := App{
		Name:      "aName",
		Framework: "django",
	}

	err := a.Create()
	c.Assert(err, IsNil)
	err = a.Destroy()
	c.Assert(err, IsNil)

	qtd, err := db.Session.Apps().Find(nil).Count()
	c.Assert(err, IsNil)
	c.Assert(qtd, Equals, 0)
}

func (s *S) TestCreate(c *C) {
	a := App{}
	a.Name = "appName"
	a.Framework = "django"

	err := a.Create()
	c.Assert(err, IsNil)

	repoPath := GetRepositoryPath(&a)
	_, err = os.Open(repoPath) // test if repository dir exists
	c.Assert(err, IsNil)

	c.Assert(a.State, Equals, "Pending")

	var retrievedApp App
	err = db.Session.Apps().Find(bson.M{"name": a.Name}).One(&retrievedApp)
	c.Assert(err, IsNil)
	c.Assert(retrievedApp.Name, Equals, a.Name)
	c.Assert(retrievedApp.Framework, Equals, a.Framework)
	c.Assert(retrievedApp.State, Equals, a.State)

	a.Destroy()

	_, err = os.Open(repoPath)
	c.Assert(err, NotNil) // ensures that repository dir has been deleted
}

func (s *S) TestCantCreateTwoAppsWithTheSameName(c *C) {
	a := App{Name: "appName", Framework: "django"}
	err := a.Create()
	c.Assert(err, IsNil)

	err = a.Create()
	c.Assert(err, NotNil)

	a.Destroy()
}

func (s *S) TestGrantAccess(c *C) {
	a := App{Name: "appName", Framework: "django", Teams: []auth.Team{}}
	err := a.GrantAccess(&s.team)
	c.Assert(err, IsNil)
	c.Assert(s.team, HasAccessTo, a)
}

func (s *S) TestGrantAccessFailsIfTheTeamAlreadyHasAccessToTheApp(c *C) {
	a := App{Name: "appName", Framework: "django", Teams: []auth.Team{s.team}}
	err := a.GrantAccess(&s.team)
	c.Assert(err, NotNil)
	c.Assert(err, ErrorMatches, "^This team has already access to this app$")
}

func (s *S) TestRevokeAccess(c *C) {
	a := App{Name: "appName", Framework: "django", Teams: []auth.Team{s.team}}
	err := a.RevokeAccess(&s.team)
	c.Assert(err, IsNil)
	c.Assert(s.team, Not(HasAccessTo), a)
}

func (s *S) TestRevokeAccessFailsIfTheTeamsDoesNotHaveAccessToTheApp(c *C) {
	a := App{Name: "appName", Framework: "django", Teams: []auth.Team{}}
	err := a.RevokeAccess(&s.team)
	c.Assert(err, NotNil)
	c.Assert(err, ErrorMatches, "^This team does not have access to this app$")
}

func (s *S) TestCheckUserAccess(c *C) {
	u := &auth.User{Email: "boy@thewho.com", Password: "123"}
	u2 := &auth.User{Email: "boy2@thewho.com", Password: "123"}
	t := auth.Team{Name: "hello", Users: []*auth.User{u}}
	a := App{Name: "appName", Framework: "django", Teams: []auth.Team{t}}
	c.Assert(a.CheckUserAccess(u), Equals, true)
	c.Assert(a.CheckUserAccess(u2), Equals, false)
}

func (s *S) TestCheckUserAccessWithMultipleUsersOnMultipleGroupsOnApp(c *C) {
	one := &auth.User{Email: "imone@thewho.com", Password: "123"}
	punk := &auth.User{Email: "punk@thewho.com", Password: "123"}
	cut := &auth.User{Email: "cutmyhair@thewho.com", Password: "123"}
	who := auth.Team{Name: "TheWho", Users: []*auth.User{one, punk, cut}}
	what := auth.Team{Name: "TheWhat", Users: []*auth.User{one, punk}}
	where := auth.Team{Name: "TheWhere", Users: []*auth.User{one}}
	a := App{Name: "appppppp", Teams: []auth.Team{who, what, where}}
	c.Assert(a.CheckUserAccess(cut), Equals, true)
	c.Assert(a.CheckUserAccess(punk), Equals, true)
	c.Assert(a.CheckUserAccess(one), Equals, true)
}
