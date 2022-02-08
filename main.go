package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	cron "github.com/robfig/cron/v3"
	"github.com/uber/go-torch/pprof"
	"github.com/uber/go-torch/renderer"
	"github.com/uber/go-torch/torchlog"

	"github.com/go-ini/ini"
	gflags "github.com/jessevdk/go-flags"
)

// options are the parameters for go-torch.
var (
	onProcessMap = map[string]int64{}
	timeDiff     = 2
	cfg          *ini.File
)

type options struct {
	PProfOptions pprof.Options `group:"pprof Options"`
	OutputOpts   outputOptions `group:"Output Options"`
}

type outputOptions struct {
	File              string `short:"f" long:"file" default:"torch.svg" description:"Output file name (must be .svg)"`
	Print             bool   `short:"p" long:"print" description:"Print the generated svg to stdout instead of writing to file"`
	Raw               bool   `short:"r" long:"raw" description:"Print the raw call graph output to stdout instead of creating a flame graph; use with Brendan Gregg's flame graph perl script (see https://github.com/brendangregg/FlameGraph)"`
	Title             string `long:"title" default:"Flame Graph" description:"Graph title to display in the output file"`
	Width             int64  `long:"width" default:"1200" description:"Generated graph width"`
	Hash              bool   `long:"hash" description:"Colors are keyed by function name hash"`
	Colors            string `long:"colors" default:"" description:"set color palette. choices are: hot (default), mem, io, wakeup, chain, java, js, perl, red, green, blue, aqua, yellow, purple, orange"`
	ConsistentPalette bool   `long:"cp" description:"Use consistent palette (palette.map)"`
	Reverse           bool   `long:"reverse" description:"Generate stack-reversed flame graph"`
	Inverted          bool   `long:"inverted" description:"icicle graph"`
	WxKey             string `long:"wxkey" description:"wx notice key"`
}

func runWithOptions(allOpts *options, remaining []string) error {
	pprofRawOutput, err := pprof.GetRaw(allOpts.PProfOptions, remaining)
	if err != nil {
		return fmt.Errorf("could not get raw output from pprof: %v", err)
	}

	profile, err := pprof.ParseRaw(pprofRawOutput)
	if err != nil {
		return fmt.Errorf("could not parse raw pprof output: %v", err)
	}

	sampleIndex := pprof.SelectSample(remaining, profile.SampleNames)
	flameInput, err := renderer.ToFlameInput(profile, sampleIndex)
	if err != nil {
		return fmt.Errorf("could not convert stacks to flamegraph input: %v", err)
	}

	opts := allOpts.OutputOpts
	if opts.Raw {
		torchlog.Print("Printing raw flamegraph input to stdout")
		fmt.Printf("%s\n", flameInput)
		return nil
	}

	var flameGraphArgs = buildFlameGraphArgs(opts)
	flameGraph, err := renderer.GenerateFlameGraph(flameInput, flameGraphArgs...)
	if err != nil {
		return fmt.Errorf("could not generate flame graph: %v", err)
	}

	if opts.Print {
		torchlog.Print("Printing svg to stdout")
		fmt.Printf("%s\n", flameGraph)
		return nil
	}

	torchlog.Printf("Writing svg to %v", opts.File)
	if err := ioutil.WriteFile(opts.File, flameGraph, 0666); err != nil {
		return fmt.Errorf("could not write output file: %v", err)
	}

	return nil
}

func validateOptions(opts *options) error {
	file := opts.OutputOpts.File
	if file != "" && !strings.HasSuffix(file, ".svg") {
		return fmt.Errorf("output file must end in .svg")
	}
	if opts.PProfOptions.TimeSeconds < 1 {
		return fmt.Errorf("seconds must be an integer greater than 0")
	}

	// extra FlameGraph options
	if opts.OutputOpts.Title == "" {
		return fmt.Errorf("flamegraph title should not be empty")
	}
	if opts.OutputOpts.Width <= 0 {
		return fmt.Errorf("flamegraph default width is 1200 pixels")
	}
	if opts.OutputOpts.Colors != "" {
		switch opts.OutputOpts.Colors {
		case "hot", "mem", "io", "wakeup", "chain", "java", "js", "perl", "red", "green", "blue", "aqua", "yellow", "purple", "orange":
			// valid
		default:
			return fmt.Errorf("unknown flamegraph colors %q", opts.OutputOpts.Colors)
		}
	}

	return nil
}

func buildFlameGraphArgs(opts outputOptions) []string {
	var args []string

	if opts.Title != "" {
		args = append(args, "--title", opts.Title)
	}

	if opts.Width > 0 {
		args = append(args, "--width", strconv.FormatInt(opts.Width, 10))
	}

	if opts.Colors != "" {
		args = append(args, "--colors", opts.Colors)
	}

	if opts.Hash {
		args = append(args, "--hash")
	}

	if opts.ConsistentPalette {
		args = append(args, "--cp")
	}

	if opts.Reverse {
		args = append(args, "--reverse")
	}

	if opts.Inverted {
		args = append(args, "--inverted")
	}

	return args
}

func helloworld(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello world"))
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	params := []string{}
	if q.Get("f") != "" {
		params = append(params, "-f", fmt.Sprintf("svg/%s.svg", q.Get("f")))
	} else {
		params = append(params, "-f", "svg/torch.svg")
	}
	if q.Get("p") != "" {
		params = append(params, "-p", q.Get("p"))
	}
	if q.Get("r") != "" {
		params = append(params, "-r", q.Get("r"))
	}
	if q.Get("title") != "" {
		params = append(params, "--title", q.Get("title"))
	}
	if q.Get("width") != "" {
		params = append(params, "--width", q.Get("width"))
	}
	if q.Get("colors") != "" {
		params = append(params, "--colors", q.Get("colors"))
	}
	if q.Get("cp") != "" {
		params = append(params, "--cp", q.Get("cp"))
	}
	if q.Get("reverse") != "" {
		params = append(params, "--reverse", q.Get("reverse"))
	}
	if q.Get("inverted") != "" {
		params = append(params, "--inverted", q.Get("inverted"))
	}

	if q.Get("u") != "" {
		params = append(params, "-u", q.Get("u"))
	}
	if q.Get("suffix") != "" {
		params = append(params, "--suffix", q.Get("suffix"))
	}
	if q.Get("b") != "" {
		params = append(params, "-b", q.Get("b"))
	}
	if q.Get("binaryname") != "" {
		params = append(params, "--binaryname", q.Get("binaryname"))
	}
	if q.Get("t") != "" {
		params = append(params, "-t", q.Get("t"))
	}
	if q.Get("pprofArgs") != "" {
		params = append(params, "--pprofArgs", q.Get("pprofArgs"))
	}
	if q.Get("wxkey") != "" {
		params = append(params, "--wxkey", q.Get("wxkey"))
	}

	opts := &options{}

	parser := gflags.NewParser(opts, gflags.Default|gflags.IgnoreUnknown)
	parser.Usage = "[options] [binary] <profile source>"
	remaining, err := parser.ParseArgs(params)
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}
	err = validateOptions(opts)
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}
	now := time.Now().Unix()
	if val, ok := onProcessMap[q.Get("u")]; ok && val > now {
		w.Write([]byte("last time wait for done"))
		return
	}
	defaultExpireSec := 30
	if q.Get("t") != "" {
		defaultExpireSec, _ = strconv.Atoi(q.Get("t"))
	}
	expireTime := defaultExpireSec + timeDiff
	onProcessMap[q.Get("u")] = now + int64(expireTime)

	go func() {
		err = runWithOptions(opts, remaining)
		if err != nil {
			torchlog.Print(err.Error())
			//w.Write([]byte(err.Error()))
			return
		}
		if opts.OutputOpts.WxKey != "" {
			sendWxTextNotice(fmt.Sprintf("压力测试：%v, 已运行完毕，火焰图地址：%s/%v", q.Get("title"), cfg.Section("").Key("HOST"), opts.OutputOpts.File), opts.OutputOpts.WxKey)
		}
	}()

	w.Write([]byte("ok"))
}

func sendWxTextNotice(content, wxkey string) {
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=%s", wxkey)
	method := "POST"
	textContent := fmt.Sprintf(`{
        "msgtype": "text",
        "text": {
            "content": "%s"
        }
  }`, content)

	payload := strings.NewReader(textContent)
	/*
	  payload := strings.NewReader(` {
	        "msgtype": "text",
	        "text": {
	            "content": "hello world"
	        }
	   }`)
	*/

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		torchlog.Print(err.Error())
		return
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		torchlog.Print(err.Error())
		return
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		torchlog.Print(err.Error())
		return
	}
	torchlog.Printf("send wx notice resp: %v", string(body))
}

func getOnProcess(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(fmt.Sprintf("%+v", onProcessMap)))
}

func delExpireData() {
	if len(onProcessMap) == 0 {
		return
	}
	now := time.Now().Unix()
	for k, v := range onProcessMap {
		if v > now {
			continue
		}
		torchlog.Printf("task %v is expired", k)
		delete(onProcessMap, k)
	}
}

func main() {
	var (
		err error
	)

	defer func() {
		if err != nil {
			torchlog.Fatalf(err.Error())
		}
	}()
	cfg, err = ini.Load("conf/conf.ini")
	if err != nil {
		return
	}
	c := cron.New()
	_, err = c.AddFunc("@every 1m", delExpireData)
	if err != nil {
		return
	}
	c.Start()

	http.Handle("/svg/", http.StripPrefix("/svg/", http.FileServer(http.Dir("svg"))))
	http.Handle("/profile/", http.StripPrefix("/profile/", http.FileServer(http.Dir(fmt.Sprintf("%s", cfg.Section("").Key("PROFLE_PATH"))))))

	http.HandleFunc("/", helloworld)
	http.HandleFunc("/tasks/", getOnProcess)
	http.HandleFunc("/pprof/", getHandler)
	torchlog.Printf("list at : %v", cfg.Section("").Key("HTTP_PORT"))
	err = http.ListenAndServe(fmt.Sprintf(":%v", cfg.Section("").Key("HTTP_PORT")), nil)
	if err != nil {
		return
	}

}
