# go-github-issues-mover

An automatic issue migration tool from one github repository to another.

## Features
- Auto create destination repository
- Auto invite all contributors from source repository
- Auto accept invitations for contributors that tool has tokens for
- Auto migrate labels
- Auto issue creation
- Auto issue comment creation

## Example config.yml
```yaml
source:
    token: asdzaf89asd7a8asd # From https://github.com/settings/tokens/new (With only "repo" scope)
    repo:
        owner: UnAfraid # Source name/organization
        name: go-github-issues-mover # Source repository name

destination:
    token: asdzaf89asd7a8asd # From https://github.com/settings/tokens/new (With only "repo" scope)
    repo:
        owner: UnAfraid # Destination name/organization
        name: go-github-issues-mover-v2 # Destination repository name
        private: true # Used only when destination repository doesn't exists, private one will be created if true, public if false
        contributors:
          unafraid: asdzaf89asd7a8asd # From https://github.com/settings/tokens/new (With only "repo" scope)
          anotherUser: zasdasdasdasdasda # From https://github.com/settings/tokens/new (With only "repo" scope)

```
