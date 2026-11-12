def normalize_labels(values: list[str]) -> list[str]:
    return sorted({value.strip().casefold() for value in values if value.strip()})
# staged note
# unstaged note
