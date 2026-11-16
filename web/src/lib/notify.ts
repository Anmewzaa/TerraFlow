import Swal from "sweetalert2";

/* Success and error messages and confirmations, as SweetAlert2. The look comes from the theme tokens
 * (see `.swal2-*` in design-system/polish.css), so it follows light and dark. */

const toast = Swal.mixin({
  toast: true,
  position: "top-end",
  showConfirmButton: false,
  timer: 4000,
  timerProgressBar: true,
  showCloseButton: true,
  didOpen: (el) => {
    el.addEventListener("mouseenter", Swal.stopTimer);
    el.addEventListener("mouseleave", Swal.resumeTimer);
  },
});

export function notifySuccess(text: string): void {
  void toast.fire({ icon: "success", title: text });
}

/** Errors stay a little longer: they are the ones a person has to read. */
export function notifyError(text: string): void {
  void toast.fire({ icon: "error", title: text, timer: 8000 });
}

export function notifyInfo(text: string): void {
  void toast.fire({ icon: "info", title: text });
}

/** A yes/no question before something that cannot be undone. Resolves true on "yes". */
export async function confirmDialog(text: string, opts: { title?: string; confirmText?: string; danger?: boolean } = {}): Promise<boolean> {
  const r = await Swal.fire({
    icon: opts.danger === false ? "question" : "warning",
    title: opts.title ?? "Are you sure?",
    text,
    showCancelButton: true,
    confirmButtonText: opts.confirmText ?? "Delete",
    cancelButtonText: "Cancel",
    focusCancel: true,
    reverseButtons: true,
  });
  return r.isConfirmed;
}

/** A message that needs reading in full, with a button to dismiss it. */
export function alertDialog(text: string, title = "Something to know"): Promise<unknown> {
  return Swal.fire({ icon: "warning", title, text, confirmButtonText: "OK" });
}
