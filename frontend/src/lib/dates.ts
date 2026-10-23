const MONTHS = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
];

/**
 * "2022-02-01" → "Feb 2022". Read off the string rather than through `Date`,
 * which parses a bare date as UTC midnight and shows the previous month to
 * anyone west of Greenwich on the 1st.
 */
export function formatMonth(date: string): string {
  const [year, month] = date.split("-");
  const name = MONTHS[Number(month) - 1];
  return name ? `${name} ${year}` : date;
}

/** A position's span: "Feb 2022 – Present", or "Start unknown" with no start. */
export function formatSpan(
  startedOn: string | null,
  endedOn: string | null,
): string {
  const start = startedOn ? formatMonth(startedOn) : "Start unknown";
  return `${start} – ${endedOn ? formatMonth(endedOn) : "Present"}`;
}
