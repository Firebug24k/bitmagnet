import {
  ChangeDetectionStrategy,
  Component,
  Input,
  booleanAttribute,
} from "@angular/core";
import { AppModule } from "../app.module";
import {
  FilterKey,
  hasActiveFilters,
  TorrentFilters,
  TorrentsSearchController,
} from "./torrents-search.controller";

export const seenPresets = [1, 3, 7, 14, 30, 90] as const;
export const seederPresets = [1, 5, 10, 50, 100] as const;

type ActiveChip = { key: FilterKey; label: string };

@Component({
  selector: "app-torrents-filters",
  standalone: true,
  imports: [AppModule],
  templateUrl: "./torrents-filters.component.html",
  styleUrl: "./torrents-filters.component.scss",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TorrentsFiltersComponent {
  @Input({ required: true }) controller: TorrentsSearchController;
  @Input({ required: true }) filters: TorrentFilters;
  @Input({ transform: booleanAttribute }) expanded = true;

  seenPresets = seenPresets;
  seederPresets = seederPresets;

  get active(): boolean {
    return hasActiveFilters(this.filters);
  }

  get activeChips(): ActiveChip[] {
    const f = this.filters;
    const chips: ActiveChip[] = [];
    if (f.seenWithinDays !== undefined) {
      chips.push({
        key: "seenWithinDays",
        label: `First seen ≤ ${formatDays(f.seenWithinDays)} ago`,
      });
    }
    if (f.seedersMin !== undefined) {
      chips.push({ key: "seedersMin", label: `Seeders ≥ ${f.seedersMin}` });
    }
    if (f.seedersMax !== undefined) {
      chips.push({ key: "seedersMax", label: `Seeders ≤ ${f.seedersMax}` });
    }
    if (f.leechersMin !== undefined) {
      chips.push({ key: "leechersMin", label: `Leechers ≥ ${f.leechersMin}` });
    }
    if (f.sizeMinGb !== undefined) {
      chips.push({ key: "sizeMinGb", label: `Size ≥ ${f.sizeMinGb} GB` });
    }
    if (f.sizeMaxGb !== undefined) {
      chips.push({ key: "sizeMaxGb", label: `Size ≤ ${f.sizeMaxGb} GB` });
    }
    return chips;
  }

  set(key: FilterKey, value: number | undefined) {
    this.controller.setFilter(key, value);
  }

  /** Toggle a preset: clicking the active preset clears it. */
  preset(key: FilterKey, value: number) {
    this.set(key, this.filters[key] === value ? undefined : value);
  }

  fromInput(key: FilterKey, event: Event, float = false) {
    const el = event.target as HTMLInputElement;
    const raw = el.value.trim();
    if (raw === "") {
      this.set(key, undefined);
      return;
    }
    const n = float ? parseFloat(raw) : parseInt(raw, 10);
    this.set(key, Number.isFinite(n) && n >= 0 ? n : undefined);
  }

  clear() {
    this.controller.clearFilters();
  }
}

const formatDays = (d: number): string =>
  d === 1 ? "24h" : d % 7 === 0 && d >= 7 ? `${d / 7}w` : `${d}d`;
