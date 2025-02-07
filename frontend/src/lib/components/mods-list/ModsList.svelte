<script lang="ts">
  import { getContextClient } from '@urql/svelte';
  import Fuse from 'fuse.js';
  import _ from 'lodash';
  import { createEventDispatcher } from 'svelte';

  import ModListFilters from './ModsListFilters.svelte';
  import ModsListItem from './ModsListItem.svelte';

  import T from '$lib/components/T.svelte';
  import VirtualList from '$lib/components/VirtualList.svelte';
  import AnnouncementsBar from '$lib/components/announcements/AnnouncementsBar.svelte';
  import { GetModCountDocument, GetModsDocument } from '$lib/generated';
  import { queuedMods } from '$lib/store/actionQueue';
  import { favoriteMods, lockfileMods, manifestMods, selectedProfileTargets } from '$lib/store/ficsitCLIStore';
  import { expandedMod, hasFetchedMods } from '$lib/store/generalStore';
  import { type OfflineMod, type PartialMod, filter, filterOptions, order, search } from '$lib/store/modFiltersStore';
  import { offline, startView } from '$lib/store/settingsStore';
  import { OfflineGetMods } from '$wailsjs/go/ficsitcli/ficsitCLI';

  const dispatch = createEventDispatcher();

  const MODS_PER_PAGE = 100;

  const client = getContextClient();

  let fetchingMods = false;
  let onlineMods: PartialMod[] = [];
  async function fetchAllModsOnline() {
    try {
      const result = await client.query(GetModCountDocument, {}, { requestPolicy: 'network-only' }).toPromise();
      const count = result.data?.getMods.count;
      if (count) {
        fetchingMods = true;
        const pages = Math.ceil(count / MODS_PER_PAGE);

        onlineMods = (await Promise.all(Array.from({ length: pages }).map(async (_, i) => {
          const offset = i * MODS_PER_PAGE;
          const modsPage = await client.query(GetModsDocument, { offset, limit: MODS_PER_PAGE }, { requestPolicy: 'network-only' }).toPromise();
          return modsPage.data?.getMods.mods ?? [];
        }))).flat();
      }
    } finally {
      fetchingMods = false;
      $hasFetchedMods = true;
    }
  }

  let offlineMods: PartialMod[] = [];
  async function fetchAllModsOffline() {
    offlineMods = (await OfflineGetMods()).map((mod) => ({
      ...mod,
      offline: true,
    } as OfflineMod));
  }
  
  let onlineRefreshInterval: number | undefined;

  $: if($offline !== null) {
    fetchAllModsOffline();
    if (!onlineRefreshInterval) {
      clearInterval(onlineRefreshInterval);
      onlineRefreshInterval = undefined;
    }
    if(!$offline) {
      fetchAllModsOnline();
      // setInterval returns NodeJS.Timeout, but that's not the case for the browser
      // eslint-disable-next-line
      // @ts-ignore
      onlineRefreshInterval = setInterval(fetchAllModsOnline, 5 * 60 * 1000); // 5 minutes
    } else {
      $hasFetchedMods = true;
    }
  }

  $: knownMods = $offline ? offlineMods : onlineMods;

  $: unknownModReferences = Object.keys($manifestMods)
    .filter((modReference) => !knownMods.find((knownMod) => knownMod.mod_reference === modReference));

  $: unknownMods = unknownModReferences.map((modReference) => {
    const offlineMod = offlineMods.find((mod) => mod.mod_reference === modReference);
    const mod = {
      mod_reference: modReference,
      name: offlineMod ? offlineMod.name : modReference,
      logo: offlineMod ? offlineMod.logo : undefined,
      authors: offlineMod ? offlineMod.authors : ['N/A'],
      missing: true,
    } as PartialMod;
    return mod;
  });

  $: mods = [...knownMods, ...unknownMods];

  let filteredMods: PartialMod[] = [];
  let filteringMods = false;
  $: {
    // Watch the required store states
    $manifestMods;
    $lockfileMods;
    $favoriteMods;
    $queuedMods;
    $selectedProfileTargets;

    filteringMods = true;
    Promise.all(mods.map((mod) => $filter.func(mod, client))).then((results) => {
      filteredMods = mods.filter((_, i) => results[i]);
    }).then(() => {
      filteringMods = false;
    });
  }

  let sortedMods: PartialMod[] = [];
  $: {
    // Watch the required store states
    $manifestMods;
    $lockfileMods;
    $favoriteMods;
    $queuedMods;
    
    sortedMods = _.sortBy(filteredMods, $order.func) as PartialMod[];
  }

  let displayMods: PartialMod[] = [];
  $: if(!$search) {
    displayMods = sortedMods;
  } else {
    const modifiedSearchString = $search.replace(/(?:author:"(.+?)"|author:([^\s"]+))/g, '="$1$2"');
    
    const fuse = new Fuse(sortedMods, {
      keys: [
        {
          name: 'name',
          weight: 2,
        },
        {
          name: 'short_description',
          weight: 1,
        },
        {
          name: 'full_description',
          weight: 0.75,
        },
        {
          name: $offline ? 'authors' : 'authors.user.username',
          weight: 0.4,
        },
      ],
      useExtendedSearch: true,
      threshold: 0.2,
      ignoreLocation: true,
    });
    displayMods = fuse.search(modifiedSearchString).map((result) => result.item);
  }

  let hasCheckedStartView = false;
  $: if($startView && mods.length > 0 && !hasCheckedStartView) {
    hasCheckedStartView = true;
    if($startView === 'expanded') {
      if(displayMods.length > 0) {
        $expandedMod = displayMods[0].mod_reference;
      }
    }
  }

  export let hideMods: boolean = false;
</script>

<div class="h-full flex flex-col">
  <div class="flex-none z-[1]">
    <ModListFilters />
  </div>
  <AnnouncementsBar />
  {#if hideMods}
    <slot />
  {:else}
    <div style="position: relative;" class="py-4 grow h-0 mods-list @container/mods-list bg-surface-200-700-token">
      <div class="mr-4 h-full flex flex-col">
        {#if fetchingMods || filteringMods}
          <div class="flex items-center justify-center">
            <div class="animate-spin rounded-full aspect-square h-8 border-t-2 border-b-2 border-primary-500"/>
          </div>
        {/if}
        {#if displayMods.length === 0 && !fetchingMods && !filteringMods && $hasFetchedMods}
          <div class="flex flex-col h-full items-center justify-center">
            {#if mods.length !== 0}
              <p class="text-xl text-center text-surface-400-700-token"><T defaultValue="No mods matching your filters" keyName="mods-list.no-mods-filtered"/></p>
              <button
                class="btn variant-filled-primary mt-4"
                on:click={() => {
                  $search = '';
                  $filter = filterOptions[0];
                }}
              >
                <T defaultValue="Show all" keyName="mods-list.show-all"/>
              </button>
            {:else}
              <p class="text-xl text-center text-surface-400-700-token"><T defaultValue="No mods found" keyName="mods-list.no-mods-found"/></p>
            {/if}
          </div>
        {:else}
          <VirtualList
            itemClass="mx-4"
            itemHeight={84}
            items={displayMods}
            let:index
            let:item={mod}>
            <ModsListItem
              {index}
              {mod}
              selected={$expandedMod == mod.mod_reference}
              on:click={() => {
                $expandedMod = mod.mod_reference;
                dispatch('expandedMod', mod.mod_reference);
              }}
            />
          </VirtualList>
        {/if}
      </div>
    </div>
  {/if}
</div>
