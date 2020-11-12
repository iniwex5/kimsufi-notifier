package order

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/ovh/go-ovh/ovh"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Cmd represents the check command
var (
	Cmd = &cobra.Command{
		Use:   "order",
		Short: "order it",
		RunE:  runner,
	}

	kimsufiUser string
	kimsufiPass string
	kimsufiKey  string
	country     string
	hardware    string
)

const (
	kimsufiAPI = ovh.OvhEU
	smsAPI     = "https://smsapi.free-mobile.fr/sendmsg"
)

func init() {
	Cmd.PersistentFlags().StringVarP(&country, "country", "c", "", "country code (e.g. fr)")
	Cmd.PersistentFlags().StringVarP(&hardware, "hardware", "w", "", "harware code name (e.g. 1801sk143)")
	Cmd.PersistentFlags().StringVarP(&kimsufiUser, "kimsufi-user", "u", "", "kimsufi api username")
	Cmd.PersistentFlags().StringVarP(&kimsufiPass, "kimsufi-pass", "p", "", "kimsufi api password")
}

func runner(cmd *cobra.Command, args []string) error {
	log.Printf("hello\n")

	u := fmt.Sprintf("https://www.kimsufi.com/fr/commande/kimsufi.xml?reference=%s", hardware)

	// create context
	execOptions := []chromedp.ExecAllocatorOption(chromedp.DefaultExecAllocatorOptions[:])
	execOptions = append(execOptions, chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64; rv:82.0) Gecko/20100101 Firefox/82.0"))
	ctx := context.Background()
	ctx, _ = chromedp.NewExecAllocator(ctx, execOptions...)
	ctx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	log.Printf("open: %s\n", u)
	err := chromedp.Run(ctx)
	if err != nil {
		return err
	}

	{
		// run task list
		ctx, _ := context.WithTimeout(ctx, 40*time.Second)
		ctx, cancel := chromedp.NewContext(ctx)
		defer cancel()
		//defer chromedp.Run(ctx, )
		err = chromedp.Run(ctx,
			chromedp.Navigate(u),
			chromedp.Click("button#header_tc_privacy_button", chromedp.NodeVisible),
			isAvailable(),
			setQuantity(1),
			setPaymentFrequency("Mensuelle"),
			login(kimsufiUser, kimsufiPass),
			selectRecurringPayement(1),
			chromedp.Click("#contracts-validation"),
			chromedp.Click("#customConractAccepted"),
			confirm(),
			waitNextPage(5*time.Second),
			fullScreenshot(90, "screenshot.png"),
		)
		if err != nil {
			return err
		}
	}

	log.Println("done")

	return nil
}

func isAvailable() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var page string
		f := chromedp.Text("#main", &page, chromedp.NodeVisible, chromedp.ByID)
		err := f.Do(ctx)
		if err != nil {
			return err
		}

		ok := strings.Contains(page, "Récapitulatif de votre commande")
		if !ok {
			return fmt.Errorf("order not available")
		}

		log.Println("order available")

		return nil
	})
}

func setQuantity(desired int) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		//var debug string
		//f := chromedp.TextContent("tbody.configuration tr.editable:nth-child(2) > td:nth-child(3)", &debug)
		//err := f.Do(ctx)
		//if err != nil {
		//	return err
		//}
		//fmt.Printf("debug\n%s\n", debug)

		var current string
		f := chromedp.TextContent("#main tbody.configuration tr.editable:nth-child(2) > td:nth-child(3)", &current)
		err := f.Do(ctx)
		if err != nil {
			return err
		}

		c, err := strconv.Atoi(current)
		if err != nil {
			return err
		}

		log.Printf("quantity: current=%d desired=%d\n", c, desired)

		if c == desired {
			return nil
		}

		f = chromedp.Click(fmt.Sprintf("#main tbody.configuration tr.editable:nth-child(2) > td:nth-child(2) > ul:nth-child(1) > li:nth-child(%d) > label:nth-child(2)", desired))
		err = f.Do(ctx)
		if err != nil {
			return err
		}

		return nil
	})
}

func setPaymentFrequency(desired string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		//var debug string
		//f := chromedp.TextContent("tbody.configuration tr.editable:nth-child(2) > td:nth-child(3)", &debug)
		//err := f.Do(ctx)
		//if err != nil {
		//	return err
		//}
		//fmt.Printf("debug\n%s\n", debug)

		var current string

		f := chromedp.TextContent("#main tbody.configuration tr.editable:nth-child(3) > td:nth-child(3)", &current)
		err := f.Do(ctx)
		if err != nil {
			return err
		}

		log.Printf("frequency: current=%s desired=%s\n", current, desired)

		if current == desired {
			return nil
		}

		f = chromedp.Click(fmt.Sprintf("//label[contains(text(),'%s')]", desired), chromedp.BySearch)
		err = f.Do(ctx)
		if err != nil {
			return err
		}

		return nil
	})
}

func login(user, pass string) chromedp.Tasks {
	messageStart := chromedp.ActionFunc(func(ctx context.Context) error {
		log.Printf("login user=%s pass=%s\n", user, strings.Repeat("x", len(pass)))
		return nil
	})
	selectLogin := chromedp.Click("#main div.customer div.you-are span.existing label")
	inputeUser := chromedp.SendKeys("#existing-customer-login", user, chromedp.ByID)
	inputePassword := chromedp.SendKeys("#existing-customer-password", pass, chromedp.ByID)
	submitLogin := chromedp.Click("div.customer-existing form span.ec-button span.middle button span", chromedp.NodeVisible)
	wait := chromedp.WaitEnabled("#contracts-validation")
	isErr := chromedp.ActionFunc(func(ctx context.Context) error {
		var page string
		f := chromedp.Text("#main", &page, chromedp.NodeVisible, chromedp.ByID)
		err := f.Do(ctx)
		if err != nil {
			return err
		}

		ok := strings.Contains(page, "Mauvais identifiant ou mot-de-passe")
		if ok {
			return fmt.Errorf("wrong crendetials")
		}

		return nil
	})
	messageEnd := chromedp.ActionFunc(func(ctx context.Context) error {
		log.Println("logged in")
		return nil
	})

	return []chromedp.Action{
		messageStart,
		selectLogin,
		inputeUser,
		inputePassword,
		submitLogin,
		isErr,
		wait,
		messageEnd,
	}
}

func selectRecurringPayement(index int) chromedp.Tasks {
	// offset by one, to skip header.
	i := index + 1
	wait := chromedp.WaitVisible(".payment-means form", chromedp.ByQuery)
	click := chromedp.Click(fmt.Sprintf(".payment-means form span:nth-child(%d) > .first > label", i), chromedp.NodeVisible, chromedp.ByQuery)
	debug := chromedp.ActionFunc(func(ctx context.Context) error {
		var payment string
		f := chromedp.Text(".payment-means form .selected > .type > label", &payment, chromedp.NodeVisible, chromedp.ByQuery)
		err := f.Do(ctx)
		if err != nil {
			return err
		}

		log.Printf("payement=%s\n", strings.ReplaceAll(payment, "\n", " - "))

		return nil
	})

	return []chromedp.Action{
		wait,
		click,
		debug,
	}
}

func confirm() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var text string
		f := chromedp.Text(".dedicated-contracts div.center:nth-child(2) button", &text)
		err := f.Do(ctx)
		if err != nil {
			return err
		}

		log.Printf("confirm=%s\n", text)

		return nil
	})
}

func waitNextPage(duration time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		//f := chromedp.WaitNotVisible(".dedicated-contracts div.center:nth-child(2) button")
		//err := f.Do(ctx)
		//if err != nil {
		//	return err
		//}

		log.Printf("sleeping=%v\n", duration)

		time.Sleep(duration)

		return nil
	})
}

// fullScreenshot takes a screenshot of the entire browser viewport.
//
// Liberally copied from puppeteer's source.
//
// Note: this will override the viewport emulation settings.
func fullScreenshot(quality int64, filename string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		// get layout metrics
		_, _, contentSize, err := page.GetLayoutMetrics().Do(ctx)
		if err != nil {
			return err
		}

		width, height := int64(math.Ceil(contentSize.Width)), int64(math.Ceil(contentSize.Height))

		// force viewport emulation
		err = emulation.SetDeviceMetricsOverride(width, height, 1, false).
			WithScreenOrientation(&emulation.ScreenOrientation{
				Type:  emulation.OrientationTypePortraitPrimary,
				Angle: 0,
			}).
			Do(ctx)
		if err != nil {
			return err
		}

		// capture screenshot
		buf, err := page.CaptureScreenshot().
			WithQuality(quality).
			WithClip(&page.Viewport{
				X:      contentSize.X,
				Y:      contentSize.Y,
				Width:  contentSize.Width,
				Height: contentSize.Height,
				Scale:  1,
			}).Do(ctx)
		if err != nil {
			return err
		}

		if err := ioutil.WriteFile(filename, buf, 0644); err != nil {
			return err
		}

		log.Printf("took screenshots: %s\n", filename)

		return nil
	})
}