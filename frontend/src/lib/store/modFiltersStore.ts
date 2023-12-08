import { get, writable } from 'svelte/store';
import type { Client } from '@urql/svelte';

import { writableBindingSync } from './wailsStoreBindings';

import { CompatibilityState, type GetModsQuery } from '$lib/generated';
import { favoriteMods, lockfileMods, manifestMods, queuedMods, selectedInstall } from '$lib/store/ficsitCLIStore';
import { GetModFiltersOrder, GetModFiltersFilter, SetModFiltersOrder, SetModFiltersFilter } from '$wailsjs/go/bindings/Settings';
import { getCompatiblity } from '$lib/utils/modCompatibility';
import type { GameBranch } from '$lib/wailsTypesExtensions';

export interface OrderBy {
  name: string;
  func: (mod: PartialMod) => unknown,
}

export interface Filter {
  name: string;
  func: (mod: PartialMod, urqlClient: Client) => Promise<boolean> | boolean,
}

export const orderByOptions: OrderBy[] = [
  { name: 'Name', func: (mod: PartialMod) => mod.name.trim() },
  { name: 'Last updated', func: (mod: PartialMod) => 'last_version_date' in mod ? Date.now() - Date.parse(mod.last_version_date) : 0 },
  { name: 'Popularity', func: (mod: PartialMod) => 'popularity' in mod ? -mod.popularity : 0 },
  { name: 'Hotness', func: (mod: PartialMod) => 'hotness' in mod ? -mod.hotness : 0 },
  { name: 'Views', func: (mod: PartialMod) => 'views' in mod ? -mod.views : 0 },
  { name: 'Downloads', func: (mod: PartialMod) => 'downloads' in mod ? -mod.downloads : 0 },
];

export const filterOptions: Filter[] = [
  { name: 'All mods', func: () => true },
  { 
    name: 'Compatible',
    func: async (mod: PartialMod, urqlClient: Client) => { 
      const installInfo = get(selectedInstall)?.info;
      if(!installInfo) {
        return false;
      }
      const gameVersion = installInfo.version;
      const branch = installInfo.branch as GameBranch;
      const compatibility = await getCompatiblity(mod.mod_reference, branch, gameVersion, urqlClient);
      return compatibility.state !== CompatibilityState.Broken;
    }, 
  },
  { name: 'Favorite', func: (mod: PartialMod) => get(favoriteMods).includes(mod.mod_reference) },
  { name: 'Queued', func: (mod: PartialMod) => get(queuedMods).some((q) => q.mod === mod.mod_reference) },
  { name: 'Installed', func: (mod: PartialMod) => mod.mod_reference in get(manifestMods) },
  { name: 'Dependency', func: (mod: PartialMod) => !(mod.mod_reference in get(manifestMods)) && mod.mod_reference in get(lockfileMods) },
  { name: 'Not installed', func: (mod: PartialMod) => !(mod.mod_reference in get(manifestMods)) },
  { name: 'Enabled', func: (mod: PartialMod) => get(manifestMods)[mod.mod_reference]?.enabled ?? mod.mod_reference in get(lockfileMods) },
  { name: 'Disabled', func: (mod: PartialMod) => mod.mod_reference in get(manifestMods) && !(mod.mod_reference in get(lockfileMods)) },
];

export type PartialSMRMod = GetModsQuery['getMods']['mods'][number];
export interface OfflineMod {
  offline: true;
  mod_reference: string;
  name: string;
  logo?: string;
  authors: string[];
}
export interface MissingMod {
  missing: true;
  mod_reference: string;
  name: string;
  logo?: string;
  authors: string[];
}
export type PartialMod = PartialSMRMod | OfflineMod | MissingMod;

export const search = writable('');
export const order = writableBindingSync(orderByOptions[1], { 
  initialGet: async () => GetModFiltersOrder().then((i) => orderByOptions.find((o) => o.name === i) || orderByOptions[1]),
  updateFunction: async (o) => SetModFiltersOrder(o.name),
});
export const filter = writableBindingSync(filterOptions[0], {
  initialGet: async () => GetModFiltersFilter().then((i) => filterOptions.find((o) => o.name === i) || filterOptions[0]),
  updateFunction: async (f) => SetModFiltersFilter(f.name),
});
