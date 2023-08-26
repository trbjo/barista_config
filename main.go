// Copyright 2018 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// simple demonstrates a simpler i3bar built using barista.
// Serves as a good starting point for building custom bars.
package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"time"

	"barista.run"
	"barista.run/bar"
	"barista.run/base/click"
	"barista.run/base/watchers/netlink"
	"barista.run/colors"
	"barista.run/format"
	"barista.run/modules/battery"
	"barista.run/modules/clock"
	"barista.run/modules/github"
	"barista.run/modules/meminfo"
	"barista.run/modules/netinfo"
	"barista.run/modules/netspeed"
	"barista.run/modules/volume"
	"barista.run/modules/volume/alsa"
	"barista.run/modules/wlan"
	"barista.run/oauth"
	"barista.run/outputs"
	"barista.run/pango"

	keyring "github.com/zalando/go-keyring"
)

type MyColor struct {
	R, G, B, A uint32
}

func (c MyColor) RGBA() (uint32, uint32, uint32, uint32) {
	return c.R, c.G, c.B, c.A
}

var (
	Accent  = MyColor{13621, 33924, 58596, 65535}
	MyRed   = MyColor{0xffff, 0, 0, 0xffff}
	MyGreen = MyColor{0, 0xffff, 0, 0xffff}
	MyBlue  = MyColor{0, 0, 0xffff, 0xffff}
)

var spacer = pango.Text(" ").XXSmall()

func truncate(in string, l int) string {
	if len([]rune(in)) <= l {
		return in
	}
	return string([]rune(in)[:l-1]) + "⋯"
}

type freegeoipResponse struct {
	Lat float64 `json:"latitude"`
	Lng float64 `json:"longitude"`
}

func whereami() (lat float64, lng float64, err error) {
	resp, err := http.Get("https://freegeoip.app/json/")
	if err != nil {
		return 0, 0, err
	}
	var res freegeoipResponse
	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return 0, 0, err
	}
	return res.Lat, res.Lng, nil
}

func setupOauthEncryption() error {
	const service = "barista-github"
	var username string
	if u, err := user.Current(); err == nil {
		username = u.Username
	} else {
		username = fmt.Sprintf("user-%d", os.Getuid())
	}
	var secretBytes []byte
	// IMPORTANT: The oauth tokens used by some modules are very sensitive, so
	// we encrypt them with a random key and store that random key using
	// libsecret (gnome-keyring or equivalent). If no secret provider is
	// available, there is no way to store tokens (since the version of
	// sample-bar used for setup-oauth will have a different key from the one
	// running in i3bar). See also https://github.com/zalando/go-keyring#linux.
	secret, err := keyring.Get(service, username)
	if err == nil {
		secretBytes, err = base64.RawURLEncoding.DecodeString(secret)
	}
	if err != nil {
		secretBytes = make([]byte, 64)
		_, err := rand.Read(secretBytes)
		if err != nil {
			return err
		}
		secret = base64.RawURLEncoding.EncodeToString(secretBytes)
		keyring.Set(service, username, secret)
	}
	oauth.SetEncryptionKey(secretBytes)
	return nil
}

func main() {
	if err := setupOauthEncryption(); err != nil {
		panic(fmt.Sprintf("Could not setup oauth token encryption: %v", err))
	}

	localtime := clock.Local().
		Output(time.Second, func(now time.Time) bar.Output {
			return outputs.Textf(
				now.Format("Mon 2 Jan 15.04"),
			)
		})

	batt := battery.All().Output(
		func(i battery.Info) bar.Output {
			if i.Status == battery.Disconnected || i.Status == battery.Unknown {
				return nil
			}
			iconName := ""
			pct := i.RemainingPct()

			if i.Status == battery.Charging {
				iconName = ""
				return outputs.Textf("%s %2d%%", iconName, pct)
			}
			switch {
			case pct < 15:
				iconName = ""
			case pct < 35:
				iconName = ""
			case pct < 65:
				iconName = ""
			case pct < 85:
				iconName = ""
			case pct < 50:
				iconName = ""
			default:
				iconName = ""
			}
			return outputs.Textf("%s %2d%%", iconName, pct)
		})

	vol := volume.New(alsa.DefaultMixer()).Output(func(v volume.Volume) bar.Output {
		if v.Mute {
			return outputs.Textf("")
		}
		iconName := ""
		pct := v.Pct()
		if pct > 66 {
			iconName = ""
		} else if pct > 33 {
			iconName = ""
		}
		return outputs.Textf("%s %2d%%", iconName, pct)
	})

	freeMem := meminfo.New().Output(func(m meminfo.Info) bar.Output {
		out := outputs.Textf(format.IBytesize(m.Available()))

		freeGigs := m.Available().Gigabytes()
		switch {
		case freeGigs < 0.5:
			out.Urgent(true)
		case freeGigs < 1:
			out.Color(MyRed)
		case freeGigs < 2:
			out.Color(MyRed)
		}
		return out
	})

	sub := netlink.Any()
	iface := sub.Get().Name
	sub.Unsubscribe()
	netSpeed := netspeed.New(iface).
		RefreshInterval(1 * time.Second).
		Output(func(s netspeed.Speeds) bar.Output {
			return outputs.Textf("%s↓ %s↑", format.Byterate(s.Rx), format.Byterate(s.Tx))
		})

	showNetInfo := make(chan bool, 1)
	wlan := wlan.Any().Output(func(i wlan.Info) bar.Output {
		if i.Connected() {
			showNetInfo <- false
			icon := outputs.Text(" ").Color(Accent)
			out := outputs.Group(icon)
			ssid := outputs.Textf(i.SSID)
			out.Append((ssid))
			return out.Glue()
		}
		showNetInfo <- true
		return nil

	})

	netInfo := netinfo.New().Output(func(s netinfo.State) bar.Output {
		shouldShow := <-showNetInfo
		if !shouldShow {
			return nil
		}
		if len(s.IPs) < 1 {
			return outputs.Text("No network").Color(colors.Scheme("bad"))
		}
		return outputs.Textf("%s: %v", s.Name, s.IPs[0])
	})

	ghNotify := github.New(os.Getenv("GITHUB_CLIENT_ID"), os.Getenv("GITHUB_CLIENT_SECRET")).
		Output(func(n github.Notifications) bar.Output {
			if n.Total() == 0 {
				return nil
			}
			icon := outputs.Text("")
			bell := outputs.Text("")

			out := outputs.Group()

			out.Append(icon)
			out.Append(spacer)
			out.Append(outputs.Textf("%d", n.Total()))

			mentions := n["mention"] + n["team_mention"]
			if mentions > 0 {
				out.Append(spacer)
				out.Append(bell)
				out.Append(outputs.Textf("%d", mentions).Urgent(true))
			}
			return out.Glue().OnClick(
				click.RunLeft("xdg-open", "https://github.com/notifications"))
		})

	panic(barista.Run(
		netSpeed,
		netInfo,
		wlan,
		vol,
		freeMem,
		ghNotify,
		batt,
		localtime,
	))
}
