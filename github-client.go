package main

import (
	"fmt"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"github.com/sirupsen/logrus"
	"strings"
	"github.com/pkg/errors"
)

func getAllRepositories(client *github.Client) (allRepositories []*github.Repository, err error) {
	opt := &github.RepositoryListOptions{
		Type:        "all",
		ListOptions: github.ListOptions{PerPage: 20},
	}

	for {
		repositories, resp, err := client.Repositories.List(ctx, "", opt)
		if err != nil {
			return nil, err
		}

		allRepositories = append(allRepositories, repositories...)
		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return allRepositories, nil
}


func getAllContributors() (allContributors []*github.Contributor, err error) {
	anon := "true"
	opt := &github.ListContributorsOptions{
		Anon:        anon,
		ListOptions: github.ListOptions{PerPage: 20},
	}

	for {
		contributors, resp, err := destinationClient.Repositories.ListContributors(ctx, config.Destination.Repo.Owner, config.Destination.Repo.Name, opt)
		if err != nil {
			return allContributors, nil
		}

		allContributors = append(allContributors, contributors...)
		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return allContributors, nil
}

func acceptInvitationsToDestinationRepo() {
	for contributor, token := range config.Destination.Repo.Contributors {
		client := github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)))

		invitations, _, err := client.Users.ListInvitations(ctx, nil)
		if err != nil {
			logrus.Fatalf("Failed to fetch invitations: %s", err)
		}

		for _, invitation := range invitations {
			if invitation.GetRepo().GetOwner().GetLogin() == config.Destination.Repo.Owner && invitation.GetRepo().GetName() == config.Destination.Repo.Name {
				client.Users.AcceptInvitation(ctx, invitation.GetID())
				logrus.Printf("Accepting invitation for %s", contributor)
			}
		}
	}
}


func getAllLabels(client *github.Client, repoOwner, repoName string) (allLabels []*github.Label, err error) {
	opt := &github.ListOptions{PerPage: 20}

	for {
		labels, resp, err := client.Issues.ListLabels(ctx, repoOwner, repoName, opt)
		if err != nil {
			return nil, err
		}

		allLabels = append(allLabels, labels...)
		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return allLabels, nil
}

func getAllIssues(client *github.Client, repoOwner, repoName string) (allIssues []*github.Issue, err error) {
	opt := &github.IssueListByRepoOptions{
		State:       "all",
		Sort:        "id",
		Direction:   "asc",
		ListOptions: github.ListOptions{PerPage: 20},
	}

	for {
		issues, resp, err := client.Issues.ListByRepo(ctx, repoOwner, repoName, opt)
		if err != nil {
			return nil, err
		}

		allIssues = append(allIssues, issues...)
		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return
}

func getAllComments(issue *github.Issue) (sourceComments []*github.IssueComment, err error) {
	opt := &github.IssueListCommentsOptions{
		Sort:        "id",
		Direction:   "asc",
		ListOptions: github.ListOptions{PerPage: 20},
	}

	for {
		comments, resp, err := sourceClient.Issues.ListComments(ctx, config.Source.Repo.Owner, config.Source.Repo.Name, issue.GetNumber(), opt)
		if err != nil {
			return nil, err
		}

		sourceComments = append(sourceComments, comments...)
		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return sourceComments, nil
}

func formatIssueBody(issue *github.Issue) string {
	return fmt.Sprintf("_From @%s on %s_\n\n%s\n\n_Copied from original issue: %s/%s#%d_", issue.GetUser().GetLogin(), issue.CreatedAt.String(), issue.GetBody(), config.Source.Repo.Owner, config.Source.Repo.Name, issue.GetNumber())
}

func formatCommentBody(comment *github.IssueComment) string {
	return fmt.Sprintf("_From @%s on %s_\n\n%s", comment.GetUser().GetLogin(), comment.CreatedAt.String(), comment.GetBody())
}

func containsContributor(login string) bool {
	for _, contributor := range allContributors {
		if strings.EqualFold(contributor.GetLogin(), login) {
			return true
		}
	}
	return false
}

func convertAssignees(issue *github.Issue) []string {
	assignees := make([]string, 0)
	for _, assignee := range issue.Assignees {
		if containsContributor(assignee.GetLogin()) {
			assignees = append(assignees, assignee.GetLogin())
		}
	}
	return assignees
}

func convertLabels(issue *github.Issue) []string {
	labels := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		labels = append(labels, label.GetName())
	}
	return labels
}

func createRepo(client *github.Client, repo *github.Repository) (*github.Repository, error) {
	repo, _, err := client.Repositories.Create(ctx, "", &github.Repository{
		Name:    &config.Destination.Repo.Name,
		Private: &config.Destination.Repo.Private,
	})

	return repo, err
}

func repoExists(client *github.Client, owner, repoName string) (err error) {
	repos, err := getAllRepositories(client)
	if err == nil {
		for _, repo := range repos {
			if strings.EqualFold(*repo.Name, repoName) {
				return nil
			}
		}
		err = errors.Errorf("Repository [%s/%s] not found", owner, repoName)
	}

	return err
}

func createLabel(label *github.Label) (*github.Label, error) {
	labelToCreate := &github.Label{
		Name:  label.Name,
		Color: label.Color,
	}

	createdLabel, _, err := destinationClient.Issues.CreateLabel(ctx, config.Destination.Repo.Owner, config.Destination.Repo.Name, labelToCreate)
	return createdLabel, err
}

func createIssue(issue *github.Issue) (*github.Issue, error) {
	body := formatIssueBody(issue)
	assignees := convertAssignees(issue)
	labels := convertLabels(issue)
	issueToCreate := &github.IssueRequest{
		Title:     issue.Title,
		Body:      &body,
		Assignees: &assignees,
		Labels:    &labels,
	}
	createdIssue, _, err := getDestinationClient(issue.GetUser().GetLogin()).Issues.Create(ctx, config.Destination.Repo.Owner, config.Destination.Repo.Name, issueToCreate)
	return createdIssue, err
}

func createComment(issue *github.Issue, comment *github.IssueComment) (*github.IssueComment, error) {
	body := formatCommentBody(comment)

	commentToCreate := &github.IssueComment{
		Body: &body,
	}

	comment, _, err := getDestinationClient(comment.GetUser().GetLogin()).Issues.CreateComment(ctx, config.Destination.Repo.Owner, config.Destination.Repo.Name, issue.GetNumber(), commentToCreate)
	return comment, err
}