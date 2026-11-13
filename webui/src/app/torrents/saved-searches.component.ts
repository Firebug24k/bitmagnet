import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  DestroyRef,
  inject,
} from "@angular/core";
import { takeUntilDestroyed } from "@angular/core/rxjs-interop";
import { ActivatedRoute, Params, Router } from "@angular/router";
import { FormControl } from "@angular/forms";
import { AppModule } from "../app.module";
import { BrowserStorageService } from "../browser-storage/browser-storage.service";

const STORAGE_KEY = "bitmagnet-saved-searches";

/** Params that describe a view rather than a search; never saved. */
const transientParams = new Set(["torrent", "tab", "page"]);

export type SavedSearch = {
  id: string;
  name: string;
  params: Params;
};

@Component({
  selector: "app-saved-searches",
  standalone: true,
  imports: [AppModule],
  templateUrl: "./saved-searches.component.html",
  styleUrl: "./saved-searches.component.scss",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SavedSearchesComponent {
  private storage = inject(BrowserStorageService);
  private route = inject(ActivatedRoute);
  private router = inject(Router);
  private cdr = inject(ChangeDetectorRef);

  searches: SavedSearch[] = this.load();
  saving = false;
  name = new FormControl("");

  constructor() {
    // re-evaluate the "current" highlight whenever the search URL changes
    this.route.queryParams
      .pipe(takeUntilDestroyed(inject(DestroyRef)))
      .subscribe(() => this.cdr.markForCheck());
  }

  private load(): SavedSearch[] {
    try {
      const raw = this.storage.get(STORAGE_KEY);
      const parsed = raw ? (JSON.parse(raw) as unknown) : [];
      return Array.isArray(parsed) ? (parsed as SavedSearch[]) : [];
    } catch {
      return [];
    }
  }

  private persist() {
    this.storage.set(STORAGE_KEY, JSON.stringify(this.searches));
    this.cdr.markForCheck();
  }

  private currentParams(): Params {
    const params = this.route.snapshot.queryParams;
    return Object.fromEntries(
      Object.entries(params).filter(
        ([k, v]) => !transientParams.has(k) && v !== undefined && v !== "",
      ),
    );
  }

  private suggestName(p: Params): string {
    const parts: string[] = [];
    if (p["query"]) parts.push(`“${decodeURIComponent(p["query"] as string)}”`);
    if (p["content_type"]) parts.push(String(p["content_type"]));
    if (p["seen_days"]) parts.push(`≤${p["seen_days"]}d`);
    if (p["seeders_min"]) parts.push(`≥${p["seeders_min"]} seeds`);
    return parts.join(" · ") || "All torrents";
  }

  startSave() {
    this.saving = true;
    this.name.setValue(this.suggestName(this.currentParams()));
  }

  confirmSave() {
    const name = (this.name.value ?? "").trim();
    if (!name) {
      return;
    }
    this.searches = [
      ...this.searches.filter((s) => s.name !== name),
      {
        id: `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`,
        name,
        params: this.currentParams(),
      },
    ];
    this.saving = false;
    this.persist();
  }

  cancelSave() {
    this.saving = false;
  }

  apply(s: SavedSearch) {
    void this.router.navigate([], {
      relativeTo: this.route,
      queryParams: s.params,
    });
  }

  remove(s: SavedSearch, event: Event) {
    event.stopPropagation();
    this.searches = this.searches.filter((x) => x.id !== s.id);
    this.persist();
  }

  isCurrent(s: SavedSearch): boolean {
    const a = this.currentParams();
    const b = s.params;
    const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
    return [...keys].every((k) => String(a[k] ?? "") === String(b[k] ?? ""));
  }
}
