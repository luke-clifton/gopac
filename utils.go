// Copyright 2014 Jack Wakefield
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

package gopac

import (
	"os"
	"net"
	"regexp"
	"strings"

	"github.com/robertkrimen/otto"
)

// https://lxr.mozilla.org/seamonkey/source/netwerk/base/src/nsProxyAutoConfig.js
var javascriptUtils string = `
 var wdays = {SUN: 0, MON: 1, TUE: 2, WED: 3, THU: 4, FRI: 5, SAT: 6};
 var months = {JAN: 0, FEB: 1, MAR: 2, APR: 3, MAY: 4, JUN: 5, JUL: 6, AUG: 7, SEP: 8, OCT: 9, NOV: 10, DEC: 11}
 function weekdayRange() {
     function getDay(weekday) {
         if (weekday in wdays) {
             return wdays[weekday];
         }
         return -1;
     }
     var date = new Date();
     var argc = arguments.length;
     var wday;
     if (argc < 1)
         return false;
     if (arguments[argc - 1] == 'GMT') {
         argc--;
         wday = date.getUTCDay();
     } else {
         wday = date.getDay();
     }
     var wd1 = getDay(arguments[0]);
     var wd2 = (argc == 2) ? getDay(arguments[1]) : wd1;
     return (wd1 == -1 || wd2 == -1) ? false
                                     : (wd1 <= wday && wday <= wd2);
 }

 function dateRange() {
     function getMonth(name) {
         if (name in months) {
             return months[name];
         }
         return -1;
     }
     var date = new Date();
     var argc = arguments.length;
     if (argc < 1) {
         return false;
     }
     var isGMT = (arguments[argc - 1] == 'GMT');
 
     if (isGMT) {
         argc--;
     }
     // function will work even without explict handling of this case
     if (argc == 1) {
         var tmp = parseInt(arguments[0]);
         if (isNaN(tmp)) {
             return ((isGMT ? date.getUTCMonth() : date.getMonth()) ==
 getMonth(arguments[0]));
         } else if (tmp < 32) {
             return ((isGMT ? date.getUTCDate() : date.getDate()) == tmp);
         } else { 
             return ((isGMT ? date.getUTCFullYear() : date.getFullYear()) ==
 tmp);
         }
     }
     var year = date.getFullYear();
     var date1, date2;
     date1 = new Date(year,  0,  1,  0,  0,  0);
     date2 = new Date(year, 11, 31, 23, 59, 59);
     var adjustMonth = false;
     for (var i = 0; i < (argc >> 1); i++) {
         var tmp = parseInt(arguments[i]);
         if (isNaN(tmp)) {
             var mon = getMonth(arguments[i]);
             date1.setMonth(mon);
         } else if (tmp < 32) {
             adjustMonth = (argc <= 2);
             date1.setDate(tmp);
         } else {
             date1.setFullYear(tmp);
         }
     }
     for (var i = (argc >> 1); i < argc; i++) {
         var tmp = parseInt(arguments[i]);
         if (isNaN(tmp)) {
             var mon = getMonth(arguments[i]);
             date2.setMonth(mon);
         } else if (tmp < 32) {
             date2.setDate(tmp);
         } else {
             date2.setFullYear(tmp);
         }
     }
     if (adjustMonth) {
         date1.setMonth(date.getMonth());
         date2.setMonth(date.getMonth());
     }
     if (isGMT) {
     var tmp = date;
         tmp.setFullYear(date.getUTCFullYear());
         tmp.setMonth(date.getUTCMonth());
         tmp.setDate(date.getUTCDate());
         tmp.setHours(date.getUTCHours());
         tmp.setMinutes(date.getUTCMinutes());
         tmp.setSeconds(date.getUTCSeconds());
         date = tmp;
     }
     return ((date1 <= date) && (date <= date2));
 }

 function timeRange() {
     var argc = arguments.length;
     var date = new Date();
     var isGMT= false
 
     if (argc < 1) {
         return false;
     }
     if (arguments[argc - 1] == 'GMT') {
         isGMT = true;
         argc--;
     }
 
     var hour = isGMT ? date.getUTCHours() : date.getHours();
     var date1, date2;
     date1 = new Date();
     date2 = new Date();
 
     if (argc == 1) {
         return (hour == arguments[0]);
     } else if (argc == 2) {
         return ((arguments[0] <= hour) && (hour <= arguments[1]));
     } else {
         switch (argc) {
         case 6:
             date1.setSeconds(arguments[2]);
             date2.setSeconds(arguments[5]);
         case 4:
             var middle = argc >> 1;
             date1.setHours(arguments[0]);
             date1.setMinutes(arguments[1]);
             date2.setHours(arguments[middle]);
             date2.setMinutes(arguments[middle + 1]);
             if (middle == 2) {
                 date2.setSeconds(59);
             }
             break;
         default:
           throw 'timeRange: bad number of arguments'
         }
     }
 
     if (isGMT) {
         date.setFullYear(date.getUTCFullYear());
         date.setMonth(date.getUTCMonth());
         date.setDate(date.getUTCDate());
         date.setHours(date.getUTCHours());
         date.setMinutes(date.getUTCMinutes());
         date.setSeconds(date.getUTCSeconds());
     }
     return ((date1 <= date) && (date <= date2));
}`

// isPlainHostName return true if there is no domain name in the host.
func isPlainHostName(host string) bool {
	return strings.Index(host, ".") == -1
}

// dnsDomainIs return true if the host is valid for the domain.
func dnsDomainIs(host, domain string) bool {
	if len(host) < len(domain) {
		return false
	}

	return strings.HasSuffix(host, domain)
}

// localHostOrDomainIs returns true if the host matches the specified hostdom,
// or if there is no domain name part in the host, but the unqualified hostdom
// matches.
func localHostOrDomainIs(host, hostdom string) bool {
	if host == hostdom {
		return true
	}

	return strings.LastIndex(hostdom, host+".") == 0
}

// isResolvable returns true if the host is resolvable.
func isResolvable(host string) bool {
	if len(host) == 0 {
		return false
	}

	if _, err := net.ResolveIPAddr("ip4", host); err != nil {
		return false
	}

	return true
}

func resolve(host string) (net.IP, error) {
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.To4(), nil
	}

	ip2, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		return nil, err
	}

	return ip2.IP, nil
}

// isInNet returns true if the IP address of the host matches the specified IP
// address pattern.
// mask is the pattern informing which parts of the IP address to match against.
// 0 means ignore, 255 means match.
func isInNet(host, pattern, mask string) bool {
	if len(host) == 0 {
		return false
	}

	address, err := resolve(host)
	if err != nil {
		return false
	}

	maskIp := net.ParseIP(mask)
	if maskIp == nil {
		return false
	}

	maskIp4 := maskIp.To4()
	if maskIp4 == nil {
		return false
	}

	maskMask := net.IPv4Mask(maskIp4[0], maskIp4[1], maskIp4[2], maskIp4[3])
	patternIp := net.ParseIP(pattern)
	return address.Mask(maskMask).Equal(patternIp)
}

// dnsResolve returns the IP address of the host.
func dnsResolve(host string) string {
	address, err := net.ResolveIPAddr("ip4", host)

	if err != nil {
		return ""
	}

	return address.String()
}

// myIpAddress returns the IP address of the host machine.
func myIpAddress() otto.Value {
	// Check the GOPAC_MYIPADDRESS environment variable and use that
	// directly if it looks like an IP address. If it does not look
	// like an IP address, assume it is an interface name, and get
	// the IP for that interface.

	env, exists := os.LookupEnv("GOPAC_MYIPADDRESS")

	if exists {
		// check if it is an IP address
		eip := net.ParseIP(env)
		if eip != nil {
			v,err := otto.ToValue(eip.String())
			if err == nil {
				return v
			}
		}

		// Assume it's an interface name
		i,err := net.InterfaceByName(env)
		if err == nil {
			addrs, err := i.Addrs()
			if err == nil {
				for _, addr := range addrs {
					ip, _, err := net.ParseCIDR(addr.String())
					if err != nil {
						continue
					}
					if ip.IsLoopback() {
						continue
					}
					if ip.IsLinkLocalUnicast() {
						continue
					}
					value, err := otto.ToValue(ip.String())
					if err != nil {
						continue
					}
					return value
				}
			}
		}


	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return otto.UndefinedValue()
	}

	// we return the *last* ip found, because it suits my usecase.
	// todo: determine a better way to figure out which IP address
	// to use.
	r := otto.UndefinedValue()

	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}
		if ip.IsLoopback() {
			continue
		}
		if ip.IsLinkLocalUnicast() {
			continue
		}
		value, err := otto.ToValue(ip.String())
		if err != nil {
			continue
		}
		r = value
	}

	return r
}

// dnsDomainLevels returns the number of domain levels in the host.
func dnsDomainLevels(host string) int {
	return strings.Count(host, ".")
}

// shExpMatch returns true if the string matches the specified shell expression.
func shExpMatch(str, shexp string) bool {
	shexp = strings.Replace(shexp, ".", "\\.", -1)
	shexp = strings.Replace(shexp, "?", ".?", -1)
	shexp = strings.Replace(shexp, "*", ".*", -1)
	matched, err := regexp.MatchString("^"+shexp+"$", str)

	return err == nil && matched
}
