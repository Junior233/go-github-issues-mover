package main

import (
	"context"
	"time"
	"strings"
	"golang.org/x/oauth2"
	"github.com/google/go-github/github"
	"github.com/spf13/pflag"
	"github.com/sirupsen/logrus"
	"github.com/mattn/go-colorable"
)

var version = "dev"

var configPath string
var printVersion bool
var config *Config
var ctx = context.Background()
var sourceClient *github.Client
var destinationClient *github.Client
var allContributors []*github.Contributor

func init() {
	pflag.StringVarP(&configPath, "config", "c", "config.yml", "The config path")
	pflag.BoolVarP(&printVersion, "version", "v", false, "print the version")
}

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: time.RFC822,
	})
	logrus.SetOutput(colorable.NewColorableStdout())

	pflag.Parse()
	if printVersion {
		logrus.Printf("github-issues-mover version: %s", version)
		return
	}

	var err error
	config, err = ReadConfig(configPath)
	if err != nil {
		logrus.Fatal(err)
	}

	logrus.Print("Configuration loaded, initializing github clients..")

	sourceClient = github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: config.Source.Token},
	)))

	if config.Source.Token == config.Destination.Token {
		destinationClient = sourceClient
	} else {
		destinationClient = github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: config.Destination.Token},
		)))
	}

	logrus.Infof("Verifying Source repository %s/%s", config.Source.Repo.Owner, config.Source.Repo.Name)
	if err := repoExists(sourceClient, config.Source.Repo.Owner, config.Source.Repo.Name); err != nil {
		logrus.Fatalf("Source repository: %s doesn't exists!", err)
	}
	_, _, err = sourceClient.Repositories.Get(ctx, config.Source.Repo.Owner, config.Source.Repo.Name)
	if err != nil {
		logrus.Fatalf("Failed to get Source Repo: %s", err)
		return
	}

	logrus.Infof("Verifying Destination repository %s/%s", config.Destination.Repo.Owner, config.Destination.Repo.Name)
	if err := repoExists(destinationClient, config.Destination.Repo.Owner, config.Destination.Repo.Name); err != nil {
		logrus.Warnf("Destination repository: %s doesn't exists, attempting to create..", err)

		repo, err := createRepo(destinationClient, &github.Repository{
			Name:    &config.Destination.Repo.Name,
			Private: &config.Destination.Repo.Private,
		})

		if err == nil {
			logrus.Info("Destination repository [%s/%s] created", config.Destination.Repo.Owner, *repo.Name)
		} else {
			logrus.Fatalf("Failed to create Destination Repo: %s", err)
			return
		}
	}
	_, _, err = destinationClient.Repositories.Get(ctx, config.Destination.Repo.Owner, config.Destination.Repo.Name)
	if err != nil {
		logrus.Fatalf("Failed to get Destination Repo: %s", err)
		return
	}

	allContributors, err = getAllContributors()
	if err != nil {
		logrus.Fatalf("Failed to get all contributors for Destination Repo: %s", err)
		return
	}

	logrus.Info("Accepting invitations for all contributors..")
	acceptInvitationsToDestinationRepo()

	sourceLabels, err := getAllLabels(sourceClient, config.Source.Repo.Owner, config.Source.Repo.Name)
	if err != nil {
		logrus.Fatalf("Failed to get all Source labels %s", err)
		return
	}
	logrus.Infof("Loaded %d Source labels", len(sourceLabels))

	destinationLabels, err := getAllLabels(destinationClient, config.Destination.Repo.Owner, config.Destination.Repo.Name)
	if err != nil {
		logrus.Fatalf("Failed to get all Destination labels %s", err)
		return
	}
	logrus.Infof("Loaded %d Destination labels", len(destinationLabels))

	logrus.Info("Verifying labels..")
	migrateLabels(sourceLabels, destinationLabels)

	logrus.Info("Verifying issues..")
	sourceIssues, err := getAllIssues(sourceClient, config.Source.Repo.Owner, config.Source.Repo.Name)
	if err != nil {
		logrus.Fatalf("Failed to get all Source issues: %s", err)
		return
	}
	logrus.Infof("Loaded %d issues to migrate from Source", len(sourceIssues))

	destinationIssues, err := getAllIssues(destinationClient, config.Destination.Repo.Owner, config.Destination.Repo.Name)
	if err != nil {
		logrus.Fatalf("Failed to get all Destination issues: %s", err)
		return
	}
	logrus.Infof("Loaded %d issues to migrate from Destination", len(destinationIssues))

	migrateIssues(sourceIssues, destinationIssues)
	logrus.Info("Done")
}

func migrateLabels(sourceLabels, destinationLabels []*github.Label) {
SOURCE:
	for _, sourceLabel := range sourceLabels {
		for _, destinationLabel := range destinationLabels {
			if strings.EqualFold(sourceLabel.GetName(), destinationLabel.GetName()) {
				continue SOURCE
			}
		}

		createdLabel, err := createLabel(sourceLabel)
		if err != nil {
			logrus.Fatalf("Failed to create label: %s - %s", sourceLabel.GetName(), err)
			continue
		}
		logrus.Infof("Created new label: %s", createdLabel.GetName())
	}
}

func migrateIssues(sourceIssues, destinationIssues []*github.Issue) {
SOURCE:
	for _, sourceIssue := range sourceIssues {
		for _, destinationIssue := range destinationIssues {
			if destinationIssue.GetTitle() == sourceIssue.GetTitle() {
				logrus.Infof("Issue already exists %s/%s#%d -> %s/%s#%d", config.Source.Repo.Owner, config.Source.Repo.Name, sourceIssue.GetNumber(), config.Destination.Repo.Owner, config.Destination.Repo.Name, destinationIssue.GetNumber())
				if sourceIssue.GetState() != destinationIssue.GetState() {
					getDestinationClient(destinationIssue.GetUser().GetLogin()).Issues.Edit(ctx, config.Destination.Repo.Owner, config.Destination.Repo.Name, destinationIssue.GetNumber(), &github.IssueRequest{
						State: sourceIssue.State,
					})
				}
				continue SOURCE
			}
		}

		// Get all comments of the Source sourceIssue
		allComments, err := getAllComments(sourceIssue)

		// Create Destination issue
		destinationIssue, err := createIssue(sourceIssue)
		if err != nil {
			logrus.Fatalf("Failed to create sourceIssue: %s/%s#%d - %s", config.Source.Repo.Owner, config.Source.Repo.Name, sourceIssue.GetNumber(), err)
			return
		}

		// Create comments
		for _, comment := range allComments {
			// Create comment
			_, err = createComment(destinationIssue, comment)
			if err != nil {
				logrus.Fatalf("Failed to create comment for issue %s/%s#%d - %s", config.Source.Repo.Owner, config.Source.Repo.Name, sourceIssue.GetNumber(), err)
			}
		}

		// Close issue if it was previously closed
		if sourceIssue.GetState() != destinationIssue.GetState() {
			getDestinationClient(destinationIssue.GetUser().GetLogin()).Issues.Edit(ctx, config.Destination.Repo.Owner, config.Destination.Repo.Name, destinationIssue.GetNumber(), &github.IssueRequest{
				State: sourceIssue.State,
			})
		}

		logrus.Infof("Issue created %s/%s#%d -> %s/%s#%d", config.Source.Repo.Owner, config.Source.Repo.Name, sourceIssue.GetNumber(), config.Destination.Repo.Owner, config.Destination.Repo.Name, destinationIssue.GetNumber())
	}
}

func getDestinationClient(contributor string) (*github.Client) {
	if token, ok := config.Destination.Repo.Contributors[strings.ToLower(contributor)]; ok {
		return github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)))
	}
	return destinationClient
}