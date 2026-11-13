package api

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider"
	"github.com/Anmewzaa/TerraFlow/server/internal/gitprovider/github"
	"github.com/Anmewzaa/TerraFlow/server/internal/store"
)

// errOwnerLostAccess is what a project answers while its repository link needs
// attention (D-061). It is an "unauthorized" provider error, so nothing is read or
// written, and the text tells the reader how it is repaired.
var errOwnerLostAccess = fmt.Errorf("%w: GitHub says the project owner can no longer access this repository; the owner must sign in with GitHub, or connect GitHub again, to verify it", gitprovider.ErrUnauthorized)

// RecheckOwnerAccess asks GitHub, with the token the person just proved themselves
// with, whether they can still reach the repository of every project they own, and
// records the answer (D-061). It is called at sign-in with GitHub and when GitHub is
// connected; the token is not kept.
//
// A project whose owner lost access is marked "needs attention" and stops reading and
// writing through the App; a later check that finds the access again clears it. If
// GitHub cannot be asked, nothing changes: not knowing is not the same as losing
// access. A person who owns no GitHub project costs no call to GitHub.
func (g *GitHubAPI) RecheckOwnerAccess(ctx context.Context, oid string, tok github.Token) error {
	if g == nil || g.App == nil || g.Store == nil || oid == "" {
		return nil
	}
	owned, err := g.Store.OwnedRepos(ctx, oid)
	if err != nil || len(owned) == 0 {
		return err
	}
	installs, err := g.App.UserInstallations(ctx, tok)
	if err != nil {
		return err
	}
	reachable := map[string]bool{} // installation ids this person can use
	for _, in := range installs {
		reachable[strconv.FormatInt(in.ID, 10)] = true
	}
	// Ask once per installation that a project uses; decide everything before writing,
	// so an error half way leaves every project as it was.
	repos := map[string]map[string]bool{}
	for _, o := range owned {
		if _, asked := repos[o.InstallationID]; asked || !reachable[o.InstallationID] {
			continue
		}
		id, err := strconv.ParseInt(o.InstallationID, 10, 64)
		if err != nil {
			continue
		}
		list, err := g.App.UserInstallationRepos(ctx, tok, id)
		if err != nil {
			return err
		}
		set := make(map[string]bool, len(list))
		for _, rp := range list {
			set[strconv.FormatInt(rp.ID, 10)] = true
		}
		repos[o.InstallationID] = set
	}
	for _, o := range owned {
		status := store.RepoAccessNeedsAttention
		if repos[o.InstallationID][o.RepoExternalID] {
			status = store.RepoAccessOK
		}
		if err := g.Store.SetRepoAccess(ctx, o.ProjectID, status); err != nil {
			return err
		}
		if status == o.AccessStatus {
			continue
		}
		action := "github.access_restored"
		if status == store.RepoAccessNeedsAttention {
			action = "github.access_lost"
		}
		_ = g.Store.LogAudit(ctx, oid, action, "project:"+o.ProjectID, map[string]any{"repository": o.RepoFullName})
	}
	return nil
}
