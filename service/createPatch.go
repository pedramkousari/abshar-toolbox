package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/pedramkousari/abshar-toolbox/helpers"
	"github.com/pedramkousari/abshar-toolbox/types"
)

type createPackage struct {
	directory string
	branch1   string
	branch2   string
	config    *helpers.ConfigService
}

var currentDirectory string
var tempDir string

var excludePath = []string{
	".env",
	"vmanager.json",
	// "database/seeds/ControlTableSeeder.php",
	// "database/seeds/SubControlTableSeeder.php",
	// "database/seeds/MgaChoicesTableSeeder.php",
	// "database/seeds/RmBaseTreatmentCollectionCategorizationTableSeeder.php",
}

func init() {
	currentDirectory, _ = os.Getwd()

	os.RemoveAll(currentDirectory + "/temp")

	err := os.Mkdir(currentDirectory+"/temp", 0755)
	if err != nil {
		if os.IsExist(err) {
			fmt.Println("The directory named", currentDirectory+"/temp", "exists")
		} else {
			log.Fatalln(err)
		}
	}
}

func CreatePackage(srcDirectory string, branch1 string, branch2 string, cnf *helpers.ConfigService) *createPackage {

	if srcDirectory == "" {
		log.Fatal("src directory not initialized")
	}

	if branch1 == "" {
		log.Fatal("branch 1 not initialized")
	}

	if branch2 == "" {
		log.Fatal("branch 2 not initialized")
	}

	createTempDirectory(srcDirectory)

	return &createPackage{
		directory: srcDirectory,
		branch1:   branch1,
		branch2:   branch2,
		config:    cnf,
	}
}

func (cr *createPackage) switchBranch() {
	cmd := exec.Command("git", "checkout", cr.branch2)
	cmd.Dir = cr.directory
	_, err := cmd.Output()
	if err != nil {
		log.Fatal(err.Error())
	}
}

func (cr *createPackage) GenerateDiffJson() {

	file, err := os.Create(tempDir + "/composer-lock-diff.json")
	if err != nil {
		panic(err)
	}

	cmd := exec.Command("composer-lock-diff", "--from", cr.branch1, "--to", cr.branch2, "--json", "--pretty", "--only-prod")
	cmd.Stdout = file
	// cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = cr.directory

	err = cmd.Run()
	if err != nil {
		panic(err)
	}
}

func (cr *createPackage) Run(ctx context.Context, progress func(types.Process)) (string, error) {
	// fmt.Println("Started ...")

	progress(types.Process{State: 0})

	cr.removeTag()

	progress(types.Process{State: 5})

	if err := cr.fetch(); err != nil {
		return "", err
	}

	progress(types.Process{State: 10})

	if err := cr.getDiffComposer(); err != nil {
		return "", err
	}
	progress(types.Process{State: 20})
	// fmt.Printf("Generated Diff.txt \n")

	// err := os.Chdir(cr.directory)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	createTarFile(cr.directory)
	progress(types.Process{State: 30})
	// fmt.Printf("Created Tar File \n")

	if composerChanged() {
		// fmt.Printf("Composer Is Change \n")

		progress(types.Process{State: 40})

		cr.switchBranch()
		// fmt.Printf("Branch Swiched  \n")
		progress(types.Process{State: 50})

		composerInstall(cr.directory, cr.config)
		// fmt.Printf("Composer Installed \n")

		progress(types.Process{State: 60})
		cr.GenerateDiffJson()
		// fmt.Printf("Generated Diff Package Composer \n")

		progress(types.Process{State: 70})
		addDiffPackageToTarFile(cr.directory)
		// fmt.Printf("Added Diff Packages To Tar File \n")

	}

	progress(types.Process{State: 80})

	copyTarFileToTempDirectory(cr.directory)
	// fmt.Printf("Moved Tar File \n")
	progress(types.Process{State: 90})

	gzipTarFile()
	// fmt.Printf("GZiped Tar File \n")
	progress(types.Process{State: 100})

	return tempDir + "/patch.tar.gz", nil
}

func (cr *createPackage) removeTag() {
	cmd := exec.Command("git", "tag", "-d", cr.branch2)
	cmd.Dir = cr.directory
	cmd.Output()
}

func (cr *createPackage) fetch() error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("git --git-dir %s/.git  fetch", cr.directory))

	_, err := cmd.Output()
	if err != nil {
		return err
	}
	return nil
}

func (cr *createPackage) getDiffComposer() error {
	// git diff --name-only --diff-filter=ACMR {lastTag} {current_tag} > diff.txt'
	// fmt.Println("git", "diff", "--name-only", "--diff-filter", "ACMR", "remotes/origin/"+cr.branch1, "remotes/origin/"+cr.branch2)
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter", "ACMR", cr.branch1, cr.branch2)

	cmd.Dir = cr.directory

	res, err := cmd.Output()
	if err != nil {
		return err
	}

	for _, path := range excludePath {
		res = []byte(strings.ReplaceAll(string(res), path, ""))
	}

	ioutil.WriteFile(tempDir+"/diff.txt", res, 0666)
	return nil
}

func createTarFile(directory string) {
	// tar -cf patch.tar --files-from=diff.txt
	cmd := exec.Command("tar", "-cf", "./patch.tar", fmt.Sprintf("--files-from=%s/diff.txt", tempDir))

	cmd.Dir = directory

	if _, err := cmd.Output(); err != nil {
		if err.Error() != "exit status 2" {
			log.Fatal(err)
		}
	}
}

func gzipTarFile() {
	// cd {baadbaan_path} && gzip -f patch.tar
	cmd := exec.Command("gzip", "-f", fmt.Sprintf("%s/patch.tar", tempDir))

	_, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
}

func composerChanged() bool {
	diffFile, err := os.Open(tempDir + "/diff.txt")
	if err != nil {
		log.Fatal(err)
	}

	defer diffFile.Close()

	scanner := bufio.NewScanner(diffFile)

	var exists bool = false

	for scanner.Scan() {
		line := scanner.Text()
		if line == "composer.lock" {
			exists = true
			break
		}
	}

	// Check for any errors during scanning
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return exists
}

func composerInstall(directory string, cnf *helpers.ConfigService) {
	safeCommand := getCommand(gitSafeDirectory, cnf)
	cmd := exec.Command(safeCommand[0], safeCommand[1:]...)
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	cmd.Dir = directory

	if err := cmd.Run(); err != nil {
		panic(err)
	}

	command := getCommand(composerInstallCommand, cnf)
	cmd = exec.Command(command[0], command[1:]...)
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	cmd.Dir = directory

	if err := cmd.Run(); err != nil {
		panic(err)
	}
}

func addDiffPackageToTarFile(directory string) {
	for packageName := range getDiffPackages() {
		cmd := exec.Command("tar", "-rf", "./patch.tar", "vendor/"+packageName)
		cmd.Dir = directory
		_, err := cmd.Output()
		if err != nil {
			log.Fatal(err.Error())
		}
	}
}

func getDiffPackages() map[string][]string {
	//TODO::remove
	file, err := os.Open(tempDir + "/composer-lock-diff.json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	type ChangesType struct {
		Changes map[string][]string `json:"changes"`
	}

	changesInstance := ChangesType{}

	if err := json.NewDecoder(file).Decode(&changesInstance); err != nil {
		log.Fatal(err)
	}

	for index, packageName := range changesInstance.Changes {
		if packageName[1] == "REMOVED" {
			delete(changesInstance.Changes, index)
		}
	}

	return changesInstance.Changes
}

func copyTarFileToTempDirectory(directory string) {
	if err := os.Rename(directory+"/patch.tar", tempDir+"/patch.tar"); err != nil {
		log.Fatal(err.Error())
	}
}

func createTempDirectory(directory string) {
	splitDir := strings.Split(directory, "/")
	tempDir = currentDirectory + "/temp/" + splitDir[len(splitDir)-1]

	err := os.Mkdir(tempDir, 0755)
	if err != nil {
		if os.IsExist(err) {
			fmt.Println("The directory named", tempDir, "exists")
		} else {
			log.Fatalln(err)
		}
	}
}

func getCommand(cmd string, cnf *helpers.ConfigService) []string {
	commandType := getCommandType(cnf)

	var command []string
	if commandType == types.DockerCommandType {
		containerName, _ := cnf.Get("CONTAINER_NAME")
		command = strings.Fields(fmt.Sprintf("docker exec %s %s", containerName, cmd))
	} else {
		command = strings.Fields(cmd)
	}

	return command
}

func getCommandType(cnf *helpers.ConfigService) types.CommandType {
	containerName, _ := cnf.Get("CONTAINER_NAME")
	commandType := types.ShellCommandType

	if containerName != "" {
		commandType = types.DockerCommandType
	}

	return commandType
}
