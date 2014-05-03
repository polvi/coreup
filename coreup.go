package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"text/tabwriter"
)

const (
	cliName        = "coreup"
	cliDescription = `Core UP!!.`
)

type Command struct {
	Name        string                  // Name of the Command and the string to use to invoke it
	Summary     string                  // One-sentence summary of what the Command does
	Usage       string                  // Usage options/arguments
	Description string                  // Detailed description of command
	Flags       flag.FlagSet            // Set of flags associated with this command
	Run         func(args []string) int // Run a command with the given arguments, return exit status
}

var (
	out           *tabwriter.Writer
	commands      []*Command
	globalFlagset *flag.FlagSet = flag.NewFlagSet(cliName, flag.ExitOnError)
	client        CoreClient

	globalFlags = struct {
		project   string
		cachePath string
		provider  string
		region    string

		image string
	}{}
)

func init() {
	out = new(tabwriter.Writer)
	out.Init(os.Stdout, 0, 8, 1, '\t', 0)
	usr, _ := user.Current()

	globalFlagset.StringVar(&globalFlags.project,
		"project",
		"coreup-"+usr.Username,
		"name for the group of servers in the same project",
	)

	globalFlagset.StringVar(&globalFlags.cachePath,
		"cred-cache",
		usr.HomeDir+"/.coreup/cred-cache.json",
		"location to store credential tokens",
	)

	globalFlagset.StringVar(&globalFlags.provider, "provider", "ec2",
		"cloud or provider to launch instance in")

	commands = []*Command{
		cmdHelp,
		cmdList,
		cmdRun,
		cmdTerminate,
	}
}

func getAllFlags() (flags []*flag.Flag) {
	return getFlags(globalFlagset)
}

func getFlags(flagset *flag.FlagSet) (flags []*flag.Flag) {
	flags = make([]*flag.Flag, 0)
	flagset.VisitAll(func(f *flag.Flag) {
		flags = append(flags, f)
	})
	return
}

func main() {
	var err error
	globalFlagset.Parse(os.Args[1:])
	var args = globalFlagset.Args()

	// no command specified - trigger help
	if len(args) < 1 {
		args = append(args, "help")
	}

	var cmd *Command

	client, err = getClient(globalFlags.project, globalFlags.provider, globalFlags.region, globalFlags.cachePath)
	if err != nil {
		fmt.Println("Unable to create client:", err)
		os.Exit(1)
	}

	// determine which Command should be run
	for _, c := range commands {
		if c.Name == args[0] {
			cmd = c
			if err := c.Flags.Parse(args[1:]); err != nil {
				fmt.Println(err.Error())
				os.Exit(2)
			}
			break
		}
	}

	if cmd == nil {
		fmt.Printf("%v: unknown subcommand: %q\n", cliName, args[0])
		fmt.Printf("Run '%v help' for usage.\n", cliName)
		os.Exit(2)
	}

	os.Exit(cmd.Run(cmd.Flags.Args()))
}
