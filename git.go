package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

func doWithGit(path string) ([]*diff, error) {
	fmt.Println()

	var commits []string
	var err error
	if len(commitsOverwrite) != 0 {
		log.Println("GIT: using internal list of commits")
		commits = commitsOverwrite
	} else {
		log.Println("GIT: getting list of commits...")
		start := time.Now()
		commits, err = gitListCommits(path)
		if err != nil {
			return nil, err
		}
		elapsed := time.Since(start)
		log.Printf("GIT: took %.2f seconds\n", elapsed.Seconds())

		if len(commits) < 2 {
			return nil, fmt.Errorf("the repo has less than 2 commits")
		}
	}

	pairs := pairCommits(commits)
	log.Printf("GIT: number of difftrees operations to perform: %d",
		len(pairs))

	log.Println("GIT: calling difftree on all commits...")
	start := time.Now()
	diffs, err := gitDiffPairs(path, pairs)
	if err != nil {
		return nil, err
	}
	elapsed := time.Since(start)
	log.Printf("GIT: took %.2f seconds\n", elapsed.Seconds())
	log.Printf("GIT: difftree speed = %.2f diffs per second\n",
		float64(len(pairs))/elapsed.Seconds())

	return diffs, nil
}

func gitListCommits(path string) ([]string, error) {
	cmd := exec.Command("git", "rev-list", "--first-parent", "HEAD")
	cmd.Dir = path

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	commits := strings.Split(string(out), "\n") // last \n adds an empty commit
	return commits[:len(commits)-1], nil        // remove it
}

func pairCommits(commits []string) [][2]string {
	ret := make([][2]string, 0, len(commits)-1)
	for i := len(commits) - 2; i >= 0; i-- {
		ret = append(ret, [2]string{commits[i+1], commits[i]})
	}
	return ret
}

func gitDiffPairs(path string, pairs [][2]string) ([]*diff, error) {
	ret := make([]*diff, 0, len(pairs))

	for i, p := range pairs {
		if i%100 == 0 {
			fmt.Print(".")
		}

		cmd := exec.Command("git", "diff-tree", "--name-status", "-r", p[0], p[1])
		cmd.Dir = path

		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf(
				"cannot diff-tree %s and %s: %s", p[0], p[1], err)
		}

		// split by lines, removing the last empty one
		lines := strings.Split(string(out), "\n")
		lines = lines[:len(lines)-1]

		ret = append(ret, &diff{
			from:  p[0],
			to:    p[1],
			lines: lines,
		})
	}
	fmt.Println()
	return ret, nil
}
