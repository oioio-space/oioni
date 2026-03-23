package netconf

import (
	"testing"

	"github.com/vishvananda/netlink"
)

type fakeNetlink struct {
	addedAddrs  []string
	addedRoutes []string
	links       []netlink.Link
}

func (f *fakeNetlink) LinkByName(name string) (netlink.Link, error) {
	for _, l := range f.links {
		if l.Attrs().Name == name {
			return l, nil
		}
	}
	la := netlink.NewLinkAttrs()
	la.Name = name
	return &netlink.Dummy{LinkAttrs: la}, nil
}
func (f *fakeNetlink) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
	f.addedAddrs = append(f.addedAddrs, addr.String())
	return nil
}
func (f *fakeNetlink) LinkSetUp(link netlink.Link) error                   { return nil }
func (f *fakeNetlink) AddrDel(link netlink.Link, addr *netlink.Addr) error { return nil }
func (f *fakeNetlink) RouteAdd(route *netlink.Route) error {
	f.addedRoutes = append(f.addedRoutes, route.Gw.String())
	return nil
}
func (f *fakeNetlink) RouteDel(route *netlink.Route) error { return nil }
func (f *fakeNetlink) LinkList() ([]netlink.Link, error)   { return f.links, nil }

func TestApplyStatic(t *testing.T) {
	nl := &fakeNetlink{}
	if err := applyStatic(nl, "eth0", "192.168.1.10/24", "192.168.1.1"); err != nil {
		t.Fatal(err)
	}
	if len(nl.addedAddrs) != 1 || nl.addedAddrs[0] != "192.168.1.10/24" {
		t.Errorf("unexpected addrs: %v", nl.addedAddrs)
	}
	if len(nl.addedRoutes) != 1 || nl.addedRoutes[0] != "192.168.1.1" {
		t.Errorf("unexpected routes: %v", nl.addedRoutes)
	}
}
