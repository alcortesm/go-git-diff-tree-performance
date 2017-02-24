package main

import (
	"fmt"
	"log"
	"time"

	git "srcd.works/go-git.v4"
	"srcd.works/go-git.v4/plumbing"
	"srcd.works/go-git.v4/plumbing/object"
)

func doWithGogit(path string) ([]*gogitResult, error) {
	fmt.Println()

	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}

	var commits []string
	if len(commitsOverwrite) != 0 {
		log.Println("GO-GIT: using internal list of commits")
		commits = commitsOverwrite
	} else {
		log.Println("GO-GIT: getting list of commits...")
		start := time.Now()
		commits, err = gogitListCommits(repo)
		if err != nil {
			return nil, err
		}
		elapsed := time.Since(start)
		log.Printf("GO-GIT: took %.2f seconds\n", elapsed.Seconds())

		if len(commits) < 2 {
			return nil, fmt.Errorf("the repo has less than 2 commits")
		}
	}

	pairs := pairCommits(commits)
	log.Printf("GO-GIT: number of difftrees operations to perform: %d",
		len(pairs))

	log.Println("GO-GIT: calling difftree on all commits...")
	start := time.Now()
	diffs, err := gogitDiffPairs(repo, pairs)
	if err != nil {
		return nil, err
	}
	elapsed := time.Since(start)
	log.Printf("GO-GIT: took %.2f seconds\n", elapsed.Seconds())
	log.Printf("GO-GIT: difftree speed = %.2f diffs per second\n",
		float64(len(pairs))/elapsed.Seconds())

	return diffs, nil
}

func gogitListCommits(r *git.Repository) ([]string, error) {
	headReference, err := r.Head()
	if err != nil {
		return nil, fmt.Errorf("cannot get the head: %s", err)
	}

	current, err := r.Commit(headReference.Hash())
	if err != nil {
		return nil, fmt.Errorf("cannot get the head commit: %s", err)
	}

	var ret []string
	var found bool
	for {
		ret = append(ret, current.Hash.String())

		if current, found = getFirstParent(current); !found {
			break
		}
	}

	return ret, nil
}

func getFirstParent(c *object.Commit) (*object.Commit, bool) {
	if c.NumParents() == 0 {
		return nil, false
	}

	iter := c.Parents()
	defer iter.Close()

	p, err := iter.Next()
	if err != nil {
		return nil, false
	}

	return p, true
}

func gogitDiffPairs(repo *git.Repository, pairs [][2]string) ([]*gogitResult, error) {
	ret := make([]*gogitResult, 0, len(pairs))
	for i, p := range pairs {
		if i%100 == 0 {
			fmt.Print(".")
		}
		diff, err := gogitDiffPair(repo, p)
		if err != nil {
			return nil, err
		}
		ret = append(ret, diff)
	}
	fmt.Println()
	return ret, nil
}

func tree(repo *git.Repository, hash string) (*object.Tree, error) {
	h := plumbing.NewHash(hash)
	c, err := repo.Commit(h)
	if err != nil {
		return nil, err
	}

	return c.Tree()
}

func gogitDiffPair(repo *git.Repository, pair [2]string) (*gogitResult, error) {
	ot, err := tree(repo, pair[0])
	if err != nil {
		return nil, err
	}

	nt, err := tree(repo, pair[1])
	if err != nil {
		return nil, err
	}

	changes, err := object.DiffTree(ot, nt)
	if err != nil {
		return nil, fmt.Errorf("cannot get changes between %s and %s: %s",
			pair[0], pair[1], err)
	}

	return &gogitResult{
		from:    pair[0],
		to:      pair[1],
		changes: changes,
	}, nil
}

type gogitResult struct {
	from    string
	to      string
	changes []*object.Change
}
