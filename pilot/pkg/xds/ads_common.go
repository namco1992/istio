// Copyright Istio Authors
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

package xds

import (
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/config/schema/gvk"
	"istio.io/istio/pkg/config/schema/resource"
)

// configKindAffectedProxyTypes contains known config types which will affect certain node types.
var configKindAffectedProxyTypes = map[resource.GroupVersionKind][]model.NodeType{
	gvk.Gateway:          {model.Router},
	gvk.QuotaSpec:        {model.SidecarProxy},
	gvk.QuotaSpecBinding: {model.SidecarProxy},
}

// ConfigAffectsProxy checks if a pushEv will affect a specified proxy. That means whether the push will be performed
// towards the proxy.
func ConfigAffectsProxy(pushEv *XdsEvent, proxy *model.Proxy) bool {
	// Empty changes means "all" to get a backward compatibility.
	if len(pushEv.configsUpdated) == 0 {
		return true
	}

	for config := range pushEv.configsUpdated {
		// If we've already know a specific configKind will affect some proxy types, check for that.
		if kindAffectedTypes, f := configKindAffectedProxyTypes[config.Kind]; f {
			for _, t := range kindAffectedTypes {
				if t == proxy.Type {
					return true
				}
			}
			continue
		}

		// Detailed config dependencies check.
		switch proxy.Type {
		case model.SidecarProxy:
			if proxy.SidecarScope.DependsOnConfig(config) {
				return true
			} else if proxy.PrevSidecarScope != nil && proxy.PrevSidecarScope.DependsOnConfig(config) {
				return true
			}
		// TODO We'll add the check for other proxy types later.
		default:
			return true
		}
	}

	return false
}

// ProxyNeedsPush check if a proxy needs push for this push event.
func ProxyNeedsPush(proxy *model.Proxy, pushEv *XdsEvent) bool {
	if ConfigAffectsProxy(pushEv, proxy) {
		return true
	}

	// If the proxy's service updated, need push for it.
	if len(proxy.ServiceInstances) > 0 && pushEv.configsUpdated != nil {
		svc := proxy.ServiceInstances[0].Service
		if _, ok := pushEv.configsUpdated[model.ConfigKey{
			Kind:      gvk.ServiceEntry,
			Name:      string(svc.Hostname),
			Namespace: svc.Attributes.Namespace,
		}]; ok {
			return true
		}
	}

	return false
}

// nolint
type XdsType int

const (
	CDS XdsType = iota
	EDS
	LDS
	RDS
)

// TODO: merge with ProxyNeedsPush
func PushTypeFor(proxy *model.Proxy, pushEv *XdsEvent) map[XdsType]bool {
	out := map[XdsType]bool{}

	// In case configTypes is not set, for example mesh configuration updated.
	// If push scoping is not enabled, we push all xds
	if len(pushEv.configsUpdated) == 0 {
		out[EDS] = true
		out[CDS] = true
		out[LDS] = true
		out[RDS] = true
		return out
	}

	// Note: CDS push must be followed by EDS, otherwise after Cluster is warmed, no ClusterLoadAssignment is retained.

	if proxy.Type == model.SidecarProxy {
		for config := range pushEv.configsUpdated {
			switch config.Kind {
			case gvk.VirtualService:
				out[LDS] = true
				out[RDS] = true
			case gvk.Gateway:
				// Do not push
			case gvk.ServiceEntry:
				out[CDS] = true
				out[EDS] = true
				out[LDS] = true
				out[RDS] = true
			case gvk.DestinationRule:
				out[CDS] = true
				out[EDS] = true
				out[RDS] = true
			case gvk.EnvoyFilter:
				out[CDS] = true
				out[EDS] = true
				out[LDS] = true
				out[RDS] = true
			case gvk.Sidecar:
				out[CDS] = true
				out[EDS] = true
				out[LDS] = true
				out[RDS] = true
			case gvk.QuotaSpec,
				gvk.QuotaSpecBinding:
				// LDS must be pushed, otherwise RDS is not reloaded
				out[LDS] = true
				out[RDS] = true
			case gvk.AuthorizationPolicy,
				gvk.RequestAuthentication:
				out[LDS] = true
			case gvk.PeerAuthentication:
				out[CDS] = true
				out[EDS] = true
				out[LDS] = true
			default:
				out[CDS] = true
				out[EDS] = true
				out[LDS] = true
				out[RDS] = true
			}
			// To return asap
			if len(out) == 4 {
				return out
			}
		}
	} else {
		for config := range pushEv.configsUpdated {
			switch config.Kind {
			case gvk.VirtualService:
				out[LDS] = true
				out[RDS] = true
			case gvk.Gateway:
				out[LDS] = true
				out[RDS] = true
			case gvk.ServiceEntry:
				out[CDS] = true
				out[EDS] = true
				out[LDS] = true
				out[RDS] = true
			case gvk.DestinationRule:
				out[CDS] = true
				out[EDS] = true
			case gvk.EnvoyFilter:
				out[CDS] = true
				out[EDS] = true
				out[LDS] = true
				out[RDS] = true
			case gvk.Sidecar,
				gvk.QuotaSpec,
				gvk.QuotaSpecBinding:
				// do not push for gateway
			case gvk.AuthorizationPolicy,
				gvk.RequestAuthentication:
				out[LDS] = true
			case gvk.PeerAuthentication:
				out[CDS] = true
				out[EDS] = true
				out[LDS] = true
			default:
				out[CDS] = true
				out[EDS] = true
				out[LDS] = true
				out[RDS] = true
			}
			// To return asap
			if len(out) == 4 {
				return out
			}
		}
	}
	return out
}
