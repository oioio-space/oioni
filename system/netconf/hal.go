// system/netconf/hal.go — injectable netlink interface for testing
package netconf

import "github.com/vishvananda/netlink"

// netlinkClient abstracts vishvananda/netlink for testing.
type netlinkClient interface {
	LinkByName(name string) (netlink.Link, error)
	AddrAdd(link netlink.Link, addr *netlink.Addr) error
	AddrDel(link netlink.Link, addr *netlink.Addr) error
	RouteAdd(route *netlink.Route) error
	RouteDel(route *netlink.Route) error
	LinkList() ([]netlink.Link, error)
}

// realNetlink delegates to vishvananda/netlink package-level functions.
type realNetlink struct{}

func (r *realNetlink) LinkByName(name string) (netlink.Link, error) { return netlink.LinkByName(name) }
func (r *realNetlink) AddrAdd(l netlink.Link, a *netlink.Addr) error  { return netlink.AddrAdd(l, a) }
func (r *realNetlink) AddrDel(l netlink.Link, a *netlink.Addr) error  { return netlink.AddrDel(l, a) }
func (r *realNetlink) RouteAdd(r2 *netlink.Route) error               { return netlink.RouteAdd(r2) }
func (r *realNetlink) RouteDel(r2 *netlink.Route) error               { return netlink.RouteDel(r2) }
func (r *realNetlink) LinkList() ([]netlink.Link, error)              { return netlink.LinkList() }
