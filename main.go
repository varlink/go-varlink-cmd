package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/TylerBrock/colorjson"
	"github.com/fatih/color"
	"github.com/varlink/go/varlink"
)

var (
	bold         = color.New(color.Bold)
	errorBoldRed string
	bridge       string
)

func errPrintf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s ", errorBoldRed)
	fmt.Fprintf(os.Stderr, format, a...)
}

func printUsage(set *flag.FlagSet, arg_help string) {
	if set == nil {
		fmt.Fprintf(os.Stderr, "Usage: %s [GLOBAL OPTIONS] COMMAND ...\n", os.Args[0])
	} else {
		fmt.Fprintf(os.Stderr, "Usage: %s [GLOBAL OPTIONS] %s [OPTIONS] %s\n", os.Args[0], set.Name(), arg_help)
	}

	fmt.Fprintln(os.Stderr, "\nGlobal Options:")
	flag.PrintDefaults()

	if set == nil {
		fmt.Fprintln(os.Stderr, "\nCommands:")
		fmt.Fprintln(os.Stderr, "  info\tPrint information about a service")
		fmt.Fprintln(os.Stderr, "  help\tPrint interface description or service information")
		fmt.Fprintln(os.Stderr, "  call\tCall a method")
	} else {
		fmt.Fprintln(os.Stderr, "\nOptions:")
		set.PrintDefaults()
	}
	os.Exit(1)
}

func varlinkCall(ctx context.Context, args []string) {
	var err error
	var oneway bool

	callFlags := flag.NewFlagSet("help", flag.ExitOnError)
	callFlags.BoolVar(&oneway, "-oneway", false, "Use bridge for connection")
	var help bool
	callFlags.BoolVar(&help, "help", false, "Prints help information")
	usage := func() { printUsage(callFlags, "<[ADDRESS/]INTERFACE.METHOD> [ARGUMENTS]") }
	callFlags.Usage = usage

	_ = callFlags.Parse(args)

	if help {
		usage()
	}

	var con *varlink.Connection
	var methodName string

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if len(bridge) != 0 {
		con, err = varlink.NewBridge(bridge)
		if err != nil {
			errPrintf("Cannot connect with bridge '%s': %v\n", bridge, err)
			os.Exit(2)
		}
		methodName = callFlags.Arg(0)
	} else {
		uri := callFlags.Arg(0)
		if uri == "" {
			usage()
		}

		li := strings.LastIndex(uri, "/")

		if li == -1 {
			errPrintf("Invalid address '%s'\n", uri)
			os.Exit(2)
		}

		address := uri[:li]
		methodName = uri[li+1:]

		con, err = varlink.NewConnection(ctx, address)
		if err != nil {
			errPrintf("Cannot connect to '%s': %v\n", address, err)
			os.Exit(2)
		}
	}
	var parameters string
	var params json.RawMessage

	parameters = callFlags.Arg(1)
	if parameters == "" {
		params = nil
	} else {
		if err := json.Unmarshal([]byte(parameters), &params); err != nil {
			errPrintf("Cannot parse parameters: %v\n", err)
			os.Exit(2)
		}
	}

	var flags uint64
	flags = 0
	if oneway {
		flags |= varlink.Oneway
	}
	recv, err := con.Send(ctx, methodName, params, flags)
	if err != nil {
		errPrintf("Error calling '%s': %v\n", methodName, err)
		os.Exit(2)
	}

	var retval map[string]interface{}

	// FIXME: Use cont
	_, err = recv(ctx, &retval)

	f := colorjson.NewFormatter()
	f.Indent = 2
	f.KeyColor = color.New(color.FgCyan)
	f.StringColor = color.New(color.FgMagenta)
	f.NumberColor = color.New(color.FgMagenta)
	f.BoolColor = color.New(color.FgMagenta)
	f.NullColor = color.New(color.FgMagenta)

	if err != nil {
		if e, ok := err.(*varlink.Error); ok {
			errPrintf("Call failed with error: %v\n", color.New(color.FgRed).Sprint(e.Name))
			errorRawParameters := e.Parameters.(*json.RawMessage)
			if errorRawParameters != nil {
				var param map[string]interface{}
				_ = json.Unmarshal(*errorRawParameters, &param)
				c, _ := f.Marshal(param)
				fmt.Fprintf(os.Stderr, "%v\n", string(c))
			}
			os.Exit(2)
		}
		errPrintf("Error calling '%s': %v\n", methodName, err)
		os.Exit(2)
	}
	c, _ := f.Marshal(retval)
	fmt.Println(string(c))
}

func varlinkHelp(ctx context.Context, args []string) {
	var err error

	helpFlags := flag.NewFlagSet("help", flag.ExitOnError)
	var help bool
	helpFlags.BoolVar(&help, "help", false, "Prints help information")
	usage := func() { printUsage(helpFlags, "<[ADDRESS/]INTERFACE>") }
	helpFlags.Usage = usage

	_ = helpFlags.Parse(args)

	if help {
		usage()
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var con *varlink.Connection
	var interfaceName string

	if len(bridge) != 0 {
		con, err = varlink.NewBridge(bridge)
		if err != nil {
			errPrintf("Cannot connect with bridge '%s': %v\n", bridge, err)
			os.Exit(2)
		}
		interfaceName = helpFlags.Arg(0)
	} else {
		uri := helpFlags.Arg(0)
		if uri == "" && bridge == "" {
			errPrintf("No ADDRESS or activation or bridge\n\n")
			usage()
		}

		li := strings.LastIndex(uri, "/")

		if li == -1 {
			errPrintf("Invalid address '%s'\n", uri)
			os.Exit(2)
		}

		address := uri[:li]

		con, err = varlink.NewConnection(ctx, address)
		if err != nil {
			errPrintf("Cannot connect to '%s': %v\n", address, err)
			os.Exit(2)
		}

		interfaceName = uri[li+1:]
	}
	description, err := con.GetInterfaceDescription(ctx, interfaceName)
	if err != nil {
		errPrintf("Cannot get interface description for '%s': %v\n", interfaceName, err)
		os.Exit(2)
	}

	fmt.Println(description)
}

func varlinkInfo(ctx context.Context, args []string) {
	var err error
	infoFlags := flag.NewFlagSet("info", flag.ExitOnError)
	var help bool
	infoFlags.BoolVar(&help, "help", false, "Prints help information")
	usage := func() { printUsage(infoFlags, "[ADDRESS]") }
	infoFlags.Usage = usage

	_ = infoFlags.Parse(args)

	if help {
		usage()
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var con *varlink.Connection
	var address string

	if len(bridge) != 0 {
		con, err = varlink.NewBridge(bridge)
		if err != nil {
			errPrintf("Cannot connect with bridge '%s': %v\n", bridge, err)
			os.Exit(2)
		}
		address = "bridge:" + bridge
	} else {
		address = infoFlags.Arg(0)

		if address == "" && bridge == "" {
			errPrintf("No ADDRESS or activation or bridge\n\n")
			usage()
		}

		con, err = varlink.NewConnection(ctx, address)
		if err != nil {
			errPrintf("Cannot connect to '%s': %v\n", address, err)
			os.Exit(2)
		}
	}

	var vendor, product, version, url string
	var interfaces []string

	err = con.GetInfo(ctx, &vendor, &product, &version, &url, &interfaces)
	if err != nil {
		errPrintf("Cannot get info for '%s': %v\n", address, err)
		os.Exit(2)
	}

	fmt.Printf("%s %s\n", bold.Sprint("Vendor:"), vendor)
	fmt.Printf("%s %s\n", bold.Sprint("Product:"), product)
	fmt.Printf("%s %s\n", bold.Sprint("Version:"), version)
	fmt.Printf("%s %s\n", bold.Sprint("URL:"), url)
	fmt.Printf("%s\n  %s\n\n", bold.Sprint("Interfaces:"), strings.Join(interfaces[:], "\n  "))
}

func main() {
	var debug bool
	var colorMode string
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flag.CommandLine.Usage = func() { printUsage(nil, "") }
	flag.BoolVar(&debug, "debug", false, "Enable debug output")
	flag.StringVar(&bridge, "bridge", "", "Use bridge for connection")
	flag.StringVar(
		&colorMode,
		"color",
		"auto",
		"colorize output [default: auto]  [possible values: on, off, auto]",
	)

	flag.Parse()

	if colorMode != "on" && (os.Getenv("TERM") == "" || colorMode == "off") {
		color.NoColor = true // disables colorized output
	}

	errorBoldRed = bold.Sprint(color.New(color.FgRed).Sprint("Error:"))

	switch flag.Arg(0) {
	case "info":
		varlinkInfo(ctx, flag.Args()[1:])
	case "help":
		varlinkHelp(ctx, flag.Args()[1:])
	case "call":
		varlinkCall(ctx, flag.Args()[1:])
	default:
		printUsage(nil, "")
	}
}
