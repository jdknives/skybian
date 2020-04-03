package imager

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/widget"
)

func (fg *FyneGUI) Page1() fyne.CanvasObject {
	title := "Welcome to Skyimager!"
	body := "This tool will:\n\n" +
		"1. Download a base image of Skybian.\n" +
		"2. Prepare an array of boot parameters.\n" +
		"3. Generate final images with provided boot parameters.\n" +
		"4. Provide instructions on how to flash final images.\n"

	conf := pageConfig{
		I:    1,
		Name: "Introduction",
		Next: func() { fg.w.SetContent(fg.Page2()) },
	}
	return makePage(fg.w, conf, widget.NewVBox(
		widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle(body, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})))
}

func (fg *FyneGUI) Page2() fyne.CanvasObject {
	wkDir := newLinkedEntry(&fg.wkDir)

	baseImgs, latestImg := fg.listBaseImgs()
	baseImg := widget.NewSelect(baseImgs, func(s string) {
		fg.baseImg = s
		fg.log.Debugf("Set: fg.baseImg = %v", s)
	})
	baseImg.Selected = fg.baseImg

	gwIP := newEntry(fg.gwIP.String(), func(s string) {
		fg.gwIP = net.ParseIP(s)
		fg.log.Debugf("Set: fg.gwIP = %v", s)
	})

	socksPC := newLinkedEntry(&fg.socksPC)
	socksPC.SetPlaceHolder("passcode")

	visors := newEntry(strconv.Itoa(fg.visors), func(s string) {
		fg.visors, _ = strconv.Atoi(s)
		fg.log.Debugf("Set: fg.visors = %v", s)
	})

	hv := widget.NewCheck("Generate Hypervisor Image.", func(b bool) {
		fg.hv = b
		fg.log.Debugf("Set: fg.hv = %v", b)
	})
	hv.SetChecked(fg.hv)

	if baseImg.Selected == "" && len(baseImg.Options) > 0 {
		baseImg.SetSelected(latestImg)
	}

	conf := pageConfig{
		I:    2,
		Name: "Prepare Boot Parameters",
		Reset: func() {
			wkDir.SetText(DefaultRootDir())
			if baseImgs, latestImg := fg.listBaseImgs(); len(baseImgs) > 0 {
				baseImg.Options = baseImgs
				baseImg.SetSelected(latestImg)
			}
			gwIP.SetText(DefaultGwIP)
			socksPC.SetText("")
			visors.SetText(strconv.Itoa(DefaultVCount))
			hv.SetChecked(true)
		},
		Check: func() error {
			wkDir, err := filepath.Abs(fg.wkDir)
			if err != nil {
				return fmt.Errorf("invalid Work Directory: %v", err)
			}
			if _, err := os.Stat(wkDir); err == nil {
				cTitle := "Work Directory Already Exists"
				cMsg := fmt.Sprintf("Directory %s already exists.\n", wkDir) +
					"Delete everything and continue?"
				dialog.ShowConfirm(cTitle, cMsg, func(b bool) {
					if !b {
						fg.w.SetContent(fg.Page2())
						return
					}
					if err := os.RemoveAll(wkDir); err != nil {
						err = fmt.Errorf("failed to clear work directory: %v", err)
						dialog.ShowError(err, fg.w)
						fg.w.SetContent(fg.Page2())
						return
					}
					dialog.ShowInformation("Information", "Work directory cleared.", fg.w)
				}, fg.w)
			}
			if strings.TrimSpace(fg.baseImg) == "" {
				return errors.New("invalid Base Image URL: cannot be empty")
			}
			if fg.gwIP == nil {
				return fmt.Errorf("invalid Gateway IP")
			}
			if _, err := strconv.Atoi(visors.Text); err != nil {
				return fmt.Errorf("invalid Number of Visor Images: %v", err)
			}
			if fg.visors < 0 {
				return fmt.Errorf("cannot create %d Visor Images", fg.visors)
			}
			return nil
		},
		Prev: func() { fg.w.SetContent(fg.Page1()) },
		Next: func() { fg.w.SetContent(fg.Page3()) },
	}
	return makePage(fg.w, conf,
		widget.NewLabel("Work Directory:"), wkDir,
		widget.NewLabel("Base Image:"), baseImg,
		widget.NewLabel("Gateway IP:"), gwIP,
		widget.NewLabel("Skysocks Passcode:"), socksPC,
		widget.NewLabel("Number of Visor Images:"), visors, hv)
}

func (fg *FyneGUI) Page3() fyne.CanvasObject {
	bps := widget.NewMultiLineEntry()
	bps.SetText(fg.generateBPS())
	conf := pageConfig{
		I:         3,
		Name:      "Finalize Boot Parameters",
		Prev:      func() { fg.w.SetContent(fg.Page2()) },
		ResetText: "Regenerate",
		Reset:     func() { bps.SetText(fg.generateBPS()) },
		NextText:  "Download and Build",
		Next: func() {
			dialog.ShowConfirm("Confirmation", "Start download and build?", func(b bool) {
				if b {
					fg.build()
				}
			}, fg.w)
		},
	}
	return makePage(fg.w, conf, bps)
}

func (fg *FyneGUI) latestBaseURL() string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	title := "Please Wait"
	msg := "Obtaining latest base image URL from GitHub..."
	d := dialog.NewProgressInfinite(title, msg, fg.w)

	d.Show()
	imgURL, err := LatestBaseImgURL(ctx, fg.log)
	d.Hide()

	if err != nil {
		dialog.ShowError(err, fg.w)
		return ""
	}
	fg.baseImg = imgURL
	return imgURL
}

func (fg *FyneGUI) listBaseImgs() ([]string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	title := "Please Wait"
	msg := "Obtaining base image releases from GitHub..."
	d := dialog.NewProgressInfinite(title, msg, fg.w)

	d.Show()
	rs, lr, err := ListReleases(ctx, fg.log)
	d.Hide()

	if err != nil {
		dialog.ShowError(err, fg.w)
		return nil, ""
	}

	fg.releases = rs
	return releaseStrings(rs), lr.String()
}