import type { MangaVolume } from "../../lib/api/manga";
export type MangaSeries = {
  key: string;
  id: number;
  title: string;
  ownerId: number;
  volumes: MangaVolume[];
};
const order = new Intl.Collator("vi", { numeric: true, sensitivity: "base" });
export function groupMangaSeries(volumes: MangaVolume[]): MangaSeries[] {
  const groups = new Map<string, MangaSeries>();
  for (const volume of volumes) {
    const key = volume.seriesId
      ? `series-${volume.seriesId}`
      : `legacy-${volume.ownerId}-${volume.classId}-${volume.title.trim().toLocaleLowerCase("vi")}`;
    let series = groups.get(key);
    if (!series) {
      series = {
        key,
        id: volume.seriesId || 0,
        title: volume.title,
        ownerId: volume.ownerId,
        volumes: [],
      };
      groups.set(key, series);
    }
    series.volumes.push(volume);
  }
  for (const series of groups.values())
    series.volumes.sort(
      (a, b) => order.compare(a.volume, b.volume) || a.id - b.id,
    );
  return [...groups.values()];
}
export function volumeLabel(
  volume: Pick<MangaVolume, "volume">,
  position: number,
) {
  return volume.volume.trim() || `Tập ${position + 1}`;
}
