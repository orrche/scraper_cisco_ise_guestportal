Go code to extend and create guest account on Cisco ISE Sponsored Guest Portal, dont know what version or if any particular setup is requried to get it to work.

Example, check if account exist if so extend it by 24 hours else create a new account.
```  golang
package main

import (
        "log"
        "time"

        "github.com/BurntSushi/toml"
        ise "github.com/orrche/scraper_cisco_ise_guestportal"
)

func main() {
        config := ise.Config{}
        _, err := toml.DecodeFile("config.toml", &config)
        if err != nil {
                log.Panic(err)
        }
        account := ise.Account{}
        _, err = toml.DecodeFile("account.toml", &account)
        if err != nil {
                log.Panic(err)
        }

        session, err := ise.CreateSession(&config)
        if err != nil {
                log.Panic(err)
        }
        defer session.Logout()

        tokens, err := session.GetAccountTokens()
        if err != nil {
                log.Panic(err)
        }

        accounts := 0
        found := false
        for _, token := range tokens {
                accounts++
                a, err := session.GetAccountData(token)
                if err != nil {
                        log.Print(err)
                        continue
                }
                if a.EmailAddress == account.EmailAddress {
                        found = true
                        log.Print(a)
                        a.FromDate = time.Now()
                        a.ToDate = a.FromDate.Add(24 * time.Hour)
                        session.UpdateAccount(a)
                        return
                }
        }

        if !found {
                account.FromDate = time.Now()
                account.ToDate = account.FromDate.Add(24 * time.Hour)
                aToken, err := session.CreateAccount(account)
                if err != nil {
                        log.Panic(err)
                }

                a, err := session.GetAccountData(aToken)
                if err != nil {
                        log.Panic(err)
                }
                log.Print(a)
        }

        log.Printf("Scanned a total of %d accounts", accounts)
}
```

account.toml - account to be created/extended
``` toml
FirstName="Kent"
LastName="Gustavsson"
EmailAddress="kent@minoris.se"
PersonBeingVisited="kent@minoris.se"
Company="company"              
```

config.toml - manager account that is used to login to the admin interface
``` toml
Username="kent"
Password="something secret"
PortalURL="https://guestportal.company.se:8443/"
Protal="722897f6-a8f6-420a-bf02-3c1f067c8e74"
```
