import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  EventEmitter,
  inject,
  Input,
  Output,
} from "@angular/core";
import { Apollo } from "apollo-angular";
import { catchError, EMPTY, finalize, map } from "rxjs";
import * as generated from "../graphql/generated";
import { ErrorsService } from "../errors/errors.service";
import { AppModule } from "../app.module";

/** Must match peerrefresh.MaxInfoHashes on the server. */
export const maxRefresh = 200;

type Row =
  generated.TorrentRefreshPeersMutation["torrent"]["refreshPeers"][number] & {
    title: string;
  };

@Component({
  selector: "app-torrents-refresh-peers",
  standalone: true,
  imports: [AppModule],
  templateUrl: "./torrents-refresh-peers.component.html",
  styleUrl: "./torrents-refresh-peers.component.scss",
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TorrentsRefreshPeersComponent {
  private apollo = inject(Apollo);
  private errorsService = inject(ErrorsService);
  private cdr = inject(ChangeDetectorRef);

  @Input({ required: true }) items: generated.TorrentContent[] = [];
  @Output() updated = new EventEmitter<null>();

  maxRefresh = maxRefresh;
  running = false;
  startedWith = 0;
  rows: Row[] = [];

  get okCount(): number {
    return this.rows.filter((r) => r.ok).length;
  }

  refresh() {
    const items = this.items.slice(0, maxRefresh);
    if (!items.length || this.running) {
      return;
    }
    const titles = new Map(items.map((i) => [i.infoHash, i.title]));
    this.running = true;
    this.startedWith = items.length;
    this.rows = [];
    this.cdr.markForCheck();
    this.apollo
      .mutate<
        generated.TorrentRefreshPeersMutation,
        generated.TorrentRefreshPeersMutationVariables
      >({
        mutation: generated.TorrentRefreshPeersDocument,
        variables: { infoHashes: items.map((i) => i.infoHash) },
      })
      .pipe(
        map((r) => r.data?.torrent.refreshPeers ?? []),
        catchError((err: Error) => {
          this.errorsService.addError(`Error refreshing peers: ${err.message}`);
          return EMPTY;
        }),
        finalize(() => {
          this.running = false;
          this.cdr.markForCheck();
        }),
      )
      .subscribe((results) => {
        this.rows = results.map((r) => ({
          ...r,
          title: titles.get(r.infoHash) ?? r.infoHash,
        }));
        this.updated.emit(null);
      });
  }

  /** "12", "?" for unknown */
  n(v?: number | null): string {
    return v === null || v === undefined ? "?" : String(v);
  }

  delta(before?: number | null, after?: number | null): number {
    return (after ?? 0) - (before ?? 0);
  }
}
