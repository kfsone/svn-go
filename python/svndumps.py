#! /usr/bin/env python3
##############################################################################
# https://github.com/kfsone/tinker/blob/master/python/svndumps
# Copyright (C) Oliver 'kfsone' Smith 2017. Licensed under the MIT License.
# See https://github.com/kfsone/tinker/blob/master/LICENSE
#
# Tool for manipulating (primarily appending) svn repository dumps.
#
##############################################################################
#

import argparse
import logging
from io import SEEK_CUR, SEEK_END
import os
from os import path as Path
import re
import sys


##############################################################################
# Constants.

FORCE_MODE = -1

revrangeRe = re.compile(r"\b(" + "r?([0-9]+)-r?([0-9]+)" + r")\b")


##############################################################################
# Logging: add our own levels for better verbosity partitioning.

# Define 3 levels of our own; note, minor info and spam.

logging._NOTE  = logging.INFO  - 3
logging._MINOR = logging._NOTE  - 3
logging._SPAM  = logging._MINOR - 3
logging.addLevelName(logging._NOTE, "NOTE")
logging.addLevelName(logging._MINOR, "MINOR")
logging.addLevelName(logging._SPAM, "SPAM")

# Alias all the logging.* and our custom levels as log_*

def log_note(*args, **kwargs):
    logging.log(logging._NOTE, *args, **kwargs)

def log_minor(*args, **kwargs):
    logging.log(logging._MINOR, *args, **kwargs)

def log_spam(*args, **kwargs):
    logging.log(logging._SPAM, *args, **kwargs)

log_info  = logging.info
log_debug = logging.debug
log_warn  = logging.warn
log_error = logging.error


##############################################################################
# Exception types.

class UsageError(UserWarning):
    """
    Class for reporting user misuse, stuff we anticipate failing that
    is not a coding error but a user error.
    """
    pass


##############################################################################
# Helpers.

def clean_path(path):
    return Path.abspath(Path.normpath(Path.expandvars(Path.expanduser(path))))


def rename_with_range(filename, old_start, old_end, new_start, new_end, dry_run=True):
    """
    Insert or replace a revision range string (nnn-mmm) into the filename
    of an existing dump file.
    """
    rangelen = max(len(str(new_end)), 6)
    fmt      = "%%0%dd" % rangelen
    new_range = (fmt+'-'+fmt) % (new_start, new_end)

    # Does the filename already contain a revision range
    rangeM = revrangeRe.search(filename)
    start_n = str(old_start)
    end_n   = str(old_end)
    end_n1  = str(old_end + 1)

    start_r = "r" + start_n
    end_r   = "r" + end_n
    end_r1  = "r" + end_n1

    patterns = [start_r, start_n, end_r, end_n, end_r1, end_n1]
    patRe, pattern = None, None
    for p in start_r, start_n, end_r, end_n, end_r1, end_n1:
        comp = re.compile(r'\b' + p + r'\b')
        if comp.search(filename):
            patRe, pattern = comp, p
            break

    msg = "rename with range %s: %d-%d to %s"
    logging.debug(msg, filename, old_start, old_end, new_range)

    new_name = filename
    if rangeM and int(rangeM.group(2)) == old_start and \
            int(rangeM.group(3)) in (old_end, old_end + 1):
        log_note("Found '%s' in old filename.", rangeM.group(1))
        new_name = filename.replace(rangeM.group(1), new_range)

    # Does it contain the start/end of the original range?
    elif patRe:
        log_note("Found '%s' in old %s.", pattern, filename)
        new_name = patRe.sub(new_range, filename)

    else:
        log_note("Inserting range into name")
        # No range, we'll add one
        base, ext = os.path.splitext(filename)
        print("base, ext = %s, %s" % (base, ext))
        new_name = base + '.' + new_range
        if ext: new_name += '.' + ext

    if new_name is filename:
        log_minor("No need to change filename.")
        return

    log_info("Renaming %s -> %s" % (filename, new_name))
    if not dry_run:
        os.rename(filename, new_name)

class Headers(list):
    pass

class Data(tuple):
    pass

def consume_revisions(filename, fh, headers=None):
    """
    Generator that will find the start and end position of every revision
    in a file starting from the current position.

    Note: Expects to be at the first 'Revision-number:' line in the file,
    does not handle consumption of the 4-line svn dump header.

    :param filename:  Name fo the file being processed (for logging etc),
    :param fh:        Open file handle to the file being processed
    :param headers:   None or a mutable list-like object to store headers

    :yield: tuple(revision_no, start_pos, end_pos)
    """
    log_debug('%s: consume revisions', filename)
    revn, start_pos, length, end_pos = None, fh.tell(), 0, None
    cur_headers = Headers()
    while True:
        line_pos = fh.tell()
        line = fh.readline()
        if not line: break
        line = line.strip()
        if not line:
            # We've reached a blank line.
            if length:
                # We've just finished parsing a header block with a
                # length value, so now we skip that much data.
                log_spam('skipping %d data bytes', length)
                if headers is not None:
                    if cur_headers: headers.append(cur_headers)
                    headers.append(Data((fh.tell(), length)))
                    cur_headers = Headers()
                fh.seek(length, SEEK_CUR)
                length = None
                assert not fh.readline().strip()

            # We should now be at the start of a new block or the end of
            # the file, or possibly another empty blank line (I'm not clear
            # on when/why there seem to be extra blanks)
            end_pos = fh.tell()

        else:
            # The only non-blank lines should be header lines.

            # split into fields
            header, _, value = line.partition(': ')

            if header == 'Revision-number':
                if end_pos:
                    log_minor('%s:r%d: ending previous block', filename, revn)
                    # If we have been tracking end_pos, that means we were
                    # consuming blank lines and are starting a new header.
                    # yield the span of the previous block.
                    save_pos = fh.tell()
                    fh.seek(end_pos)
                    if cur_headers and headers is not None: headers.append(cur_headers)
                    yield revn, start_pos, end_pos
                    fh.seek(save_pos)
                else:
                    end_pos = start_pos

                # remember the revision number
                new_revn = int(value)
                log_note('%s: revision: r%d', filename, new_revn)
                if revn and revn is not FORCE_MODE:
                    gap = new_revn - revn
                    if gap > 1:
                        if gap == 2:
                            err = "MISSING/MISSED REVISION: r%d"
                            log_warn(err % (revn + 1))
                        else:
                            err = "Gap in revisions! last: r%d, next: r%d"
                            log_warn(err % (revn, new_revn))
                revn = new_revn
                start_pos, end_pos = int(end_pos), None

                cur_headers = Headers()

            elif header == "Node-path":
                if cur_headers and headers is not None: headers.append(cur_headers)
                log_minor("%s:r%d: -- node %s", filename, revn, value)
                cur_headers = Headers()

            elif header.endswith('-length'):
                # find the longest (outer) length description
                value = int(value)
                if value > length:
                    log_debug("%s:r%d: block length? %d", filename, revn, value)
                    length = value

            cur_headers.append((header, value, line_pos))

    end_pos = fh.tell()
    log_spam("%s: END OF FILE.", filename)
    if revn and start_pos < end_pos:
        if cur_headers and headers is not None: headers.append(cur_headers)
        log_minor("%s:r%d: finish block %d-%d", filename, revn, start_pos, end_pos)
        yield revn, start_pos, end_pos


##############################################################################
# Minor functions

def get_first_revn(filename, fh):
    """
        Confirm that fh looks like an svn dump and return the first revision.

        \param  filename    Name of the file being read
        \param  fh          Open handle to the file
    """
    for line_no, exp in enumerate(("SVN-fs-dump-format-version:", "", "UUID:", "", "Revision-number:"), 1):
        last_line_pos = fh.tell()
        line = fh.readline().strip()
        log_spam('%s:%d: gfr: %s', filename, line_no, line)
        # If expect is not empty, the line should start with it.
        # If expect is empty, the line should be too.
        if (exp and not line.startswith(exp)) or (line and not exp):
            err = "%s:%d: Not recognized as an SVN dump: '%s'"
            raise RuntimeError(err % (filename, line_no, line))

    # Put the line back into the buffer for consumption.
    fh.seek(last_line_pos)

    # The last line should be a revision number:
    _, _, revn = line.partition(': ')
    return int(revn)


def get_last_revn(filename, fh):
    """
    Returns the last revision number in a file by reading all the
    revisions in the file (skips data for speed, but has to read all
    the headers.

    Note: This is a pessimistic approach, we could just read backwards
    from the end of the file, the problem with that approach is that it
    wouldn't work if your file contained 'Revision-number: ...'.

    :param filename:  Name of the file being read
    :param fh:        Open file handle to the file
    """
    for revn, start_pos, end_pos in consume_revisions(filename, fh):
        last_revn = revn
    return last_revn


##############################################################################
# Open a file or return stdin/stdout etc

def open_or_sys(filename, sysfile, mode):
    """
    Open a file by name or, if the name is '-', use a stdin/stdout file.

    :param filename: Name of the file to open,
    :param sysfile: File to use if filename is '-',
    :param mode: The open mode to use (e.g 'rb'),
    """
    return sysfile if filename == '-' else open(filename, mode)


##############################################################################
# Simple dump of file info

def dump_info(filename, strict=False, verbose=False):
    """
    Output the first/last revision contained in a file.

    :param filename: Name of the svn dump file to inspect.
    :param strict: Set True to fail on inconsistencies.
    :param verbose: Output more info
    """
    verbose = int(verbose)
    headers = [] if verbose > 1 else None

    fh = open_or_sys(filename, sys.stdin, 'rb')

    first_revn = get_first_revn(filename, fh)

    # We haven't *consumed* this revision yet, so we're going to see
    # it again immediately when we start consume_revisions.
    last_revn = first_revn - 1

    for revn, start, end in consume_revisions(filename, fh, headers=headers):
        if revn != last_revn + 1:
            error = "Got revision %d after %d!"
            if strict:
                raise UserWarning(error % (revn, last_revn))
            else:
                log_warn(error, revn, last_revn)
        if verbose:
            print("{:-7d} {:25,} bytes".format(revn, end-start))
        if verbose > 1:
            max_width = max(len(t[0]) for t in headers)
            for tup in headers:
                print("{:<{wid}s} {}".format(tup[0], tup[1], wid=max_width))
            print()

        last_revn = revn
        if headers is not None: headers[:] = []

    print("%s: %d-%d" % (filename, first_revn, last_revn))

    fh.flush()
    fh.close()


##############################################################################
# Duplicate a range of revisions with fake content.

def dump_sparse(filename, end_revn=None, strict=False):
    """
    Reads an input file and outputs a comparable dump with the content
    of files scrambled.

    :param filename: Name of the svn dump file to inspect.
    """
    headers = []

    fh = open_or_sys(filename, sys.stdin, 'rb')

    first_revn = get_first_revn(filename, fh)

    # Copy the dump headers.
    pos = fh.tell()
    fh.seek(0)
    prefix = fh.read(pos)
    print(prefix, end='')

    # We haven't *consumed* this revision yet, so we're going to see
    # it again immediately when we start consume_revisions.
    last_revn = first_revn - 1

    for revn, start, end in consume_revisions(filename, fh, headers=headers):
        if revn != last_revn + 1:
            error = "Got revision %d after %d!"
            if strict:
                raise UserWarning(error % (revn, last_revn))
            else:
                log_warn(error, revn, last_revn)

        # headers[0] will be the revision info, headers[1] will be
        # property info.
        for k, v, l in headers[0]:
            print(k + ': ' + str(v))
        print()
        fh.seek(headers[1][0])
        print(fh.read(headers[1][1]))

        if end_revn and revn > end_revn:
            while fh.tell() < end:
                bytes_to_read = min(end - fh.tell(), 16384)
                data = fh.read(bytes_to_read)
                print(data, end='')

        else:
            max_len, fake_data = 64, "SPARSE\n"
            for data in headers[2:]:
                if isinstance(data, Headers):
                    hdrs = {}
                    props_len, text_shrink, text_len = 0, 0, 0
                    for k, v, l in data:
                        hdrs[k] = v
                        if k == "Prop-content-length":
                            v = props_len = int(v)
                        elif k == "Text-content-length":
                            v = text_len = int(v)
                            if text_len > max_len:
                                v = len(fake_data)
                                text_shrink = text_len - v
                        elif k == "Content-length":
                            v = int(v)
                            if text_shrink:
                                v -= text_shrink
                        print(k + ': ' + str(v))
                    print()
                else:
                    if props_len:
                        fh.seek(data[0])
                        print(fh.read(props_len), end='')
                    if hdrs['Node-kind'] == 'file':
                        if text_shrink:
                            print(fake_data)
                        else:
                            print(fh.read(text_len), end='')
                    print()
                    print()

        last_revn = revn
        headers[:] = []


##############################################################################
# Append one dump to the end of another.
#

def _append_check_revisions(dumpfile, rh, lhs_revn, rhs_revn, strict):
    err = "%s: Previous dump ended at %d, new dump starts at %d."
    if strict:
        err += " [--strict]"
        raise UsageError(err % (dumpfile, lhs_revn, rhs_revn))
    if rhs_revn > lhs_revn + 1:
        err += " Can't join with missing revisions."
        raise UsageError(err % (dumpfile, lhs_revn, rhs_revn))

    err += " (skipping overlapping revisions)"
    log_warn(err % (dumpfile, lhs_revn, rhs_revn))

    # Consume revisions until the revision we consume is the
    # last revision we saw in the previous file
    for revn, _, end_pos in consume_revisions(dumpfile, rh):
        log_info("-- Skipped")
        if revn == lhs_revn:
            break

    # did we exhaust
    if revn < lhs_revn:
        err = "%s: covered r%d-%d, nothing to append to previous r%d"
        log_warn(err % (dumpfile, rhs_revn, revn, lhs_revn))


def append_dump(lhfilename, lh, lhs_revn, dumpfile,
                buffer_size=16384, strict=False, dry_run=False):
    """
    Append the contents of the file named by 'dumpfile' to the end of the
    left-hand side file.

    :param lhfilename: Name of the base file
    :param lh: Open file descriptor of the left-hand file
    :param lhs_revn: Last revision of lh or FORCE_MODE to force 'blind' append
    :param dumpfile: Name of the file to append
    :param buffer_size: Maximum bytes per read/write operation during copy
    :param strict: Set True to reject dumpfile if it does not start with
                   lhs_revn + 1
    :param dry_run: Set True to prevent write/modify operations
    """
    log_note("Appending %s to %s after revn %d", dumpfile, lhfilename, lhs_revn)

    revn = lhs_revn
    lh.seek(0, SEEK_END)

    rh = open_or_sys(dumpfile, sys.stdin, 'rb')
    try:

        rhs_revn = get_first_revn(dumpfile, rh)
        log_minor("%s: starts at r%d", dumpfile, rhs_revn)

        if lhs_revn is not FORCE_MODE and rhs_revn != lhs_revn + 1:
            _append_check_revisions(dumpfile, rh, lhs_revn, rhs_revn, strict)

        # now copy revisions across. This way we can tell the user what
        # the last revision is.
        for revn, start_pos, end_pos in consume_revisions(dumpfile, rh):

            length = end_pos - start_pos
            size = "{:,}".format(length)
            msg = 'appending %s:r%d (%s bytes)'
            log_note(msg, dumpfile, revn, size)

            rh.seek(start_pos)

            pos = start_pos
            while pos < end_pos:
                buf = rh.read(min(buffer_size, end_pos - pos))
                if not dry_run:
                    lh.write(buf)
                pos += len(buf)

            assert rh.tell() == end_pos

    finally:
        rh.close()

    return rhs_revn, revn


##############################################################################

def append_dumps(lhs, continuations, unlink=False, strict=False, force=False,
                 rename=False, buffer_size=None, dry_run=True):
    """
    Incrementally append one or more dumps (continuations) to a base.

    For example, you could append dumps for r20-29 and r30-300 to a file that
    ends with dump 19.

    :param lhs:  Name of the left-hand side (base) file,
    :param continuations: List of filenames to be appended,
    :param unlink: True if you want the continuation files removed after they
                   have been appended to the base.
    :param force:  True if you want to ignore revision numbers of the base
                   file. Use if you are absolutely certain that each file
                   starts with the next revision from the previous file.
    :param rename: Set true to insert or replace revision ranges in file names.
    :param strict: True to require continuations to be exactly contiguous. If
                   this is not set, overlapping revisions will be ignored.
                   E.g. when strict=False you can merge 1-100 with 100-101,
                   without duplicating 100. If strict=True this would error.
    :param dry_run: Enable to prevent any write/modify operations.
    """
    if lhs == '-':
        raise ValueError("Cannot use '-' as the source file for append")

    log_info("Appending %s to %s", continuations, lhs)
    if dry_run:
        log_note("Dry Run")

    log_debug("Opening %s", lhs)
    lh = open(lhs, 'rb+')

    try:
        # Summarize the first file if the user didn't --force, or
        # if we are in strict mode.
        lhs_start = get_first_revn(lhs, lh)
        log_note("%s: First rev: %d", lhs, lhs_start)

        if strict or (not force):
            original_end = lhs_end = get_last_revn(lhs, lh)
            log_note("%s: Last rev: %d", lhs, lhs_end)

            log_info("%s: %d-%d", lhs, lhs_start, lhs_end)

        # In force mode, lhs_end has a special value
        if force:
            original_end, lhs_end = FORCE_MODE, FORCE_MODE
            log_info("%s: Forced append", lhs)

        for dump in continuations:
            rhs_start, rhs_end = append_dump(lhs, lh, lhs_end, dump,
                                             strict=strict, dry_run=dry_run)

            if not force:
                log_info('%s now spans %d-%d', lhs, lhs_start, rhs_end)
                lhs_end = rhs_end
            else:
                msg = "%s: appended r%d-%d from %s"
                log_info(msg, lhs, rhs_start, rhs_end, dump)
                if original_end == FORCE_MODE:
                    original_end = rhs_start - 1
                lhs_end = rhs_end

            if unlink:
                log_note("Unlink: %s", dump)
                # Make sure lh is committed to disk.
                lh.flush()
                if not dry_run:
                    os.unlink(dump)
    finally:
        lh.flush()
        lh.close()

    if rename:
        rename_with_range(lhs, lhs_start, original_end, lhs_start, lhs_end,
                          dry_run=dry_run)


##############################################################################
#

def emit_range(filename, fh, firstRevn, lastRevn, strict=False):
    # Copy any header in the file.
    start_pos = fh.tell()
    start_rev = get_first_revn(filename, fh)

    if strict and start_rev > firstRevn:
        msg = "[--strict] first revn is %d but dump starts with %d"
        raise UsageError(msg % (firstRevn, start_rev))

    # Output the header from the source.
    end_pos = fh.tell()
    fh.seek(start_pos)
    sys.stdout.write(fh.read(end_pos - start_pos))
    log_debug("Transferred %d bytes of header" % (end_pos - start_pos))

    for revn, start_pos, end_pos in consume_revisions(filename, fh):
        if revn > lastRevn:
            break
        if revn < firstRevn:
            continue
        fh.seek(start_pos)
        sys.stdout.write(fh.read(end_pos - start_pos))
        log_info("Rev %d (%d bytes)" % (revn, end_pos - start_pos))

    if strict and revn < lastRevn:
        msg = "[--strict] last revn is %d but dump ended with %d"
        raise UsageError(msg % (lastRevn, revn))
    
    log_debug("Finished")


##############################################################################
# Argument parsing

def add_cmd_parser(sub_parsers, name, helptxt, func, writes):
    parser = sub_parsers.add_parser(name, help=helptxt)

    if writes:
        parser.add_argument('--dry-run', '-n', action="store_true",
                help="Dry run (read operations only)")

    parser.add_argument("--verbose", "-v",     action="count", default=0,
                help="Increase verbosity")

    parser.set_defaults(func=func)

    return parser


def parse_args(argv):
    def join_dumps_wrapper(args):
        args.base = clean_path(args.base)
        args.continuations = [clean_path(p) for p in args.continuations]

        assert args.buf_size > 0

        if args.force and args.rename:
            msg = "using --rename + --force may produce non-contiguous dumps"
            logging.warn(msg)

        append_dumps(args.base, args.continuations,
                     buffer_size=args.buf_size,
                     dry_run=args.dry_run,
                     force=args.force,
                     rename=args.rename,
                     strict=args.strict,
                     unlink=args.rm)

    def dump_info_wrapper(args):
        for filename in args.dumpfiles:
            dump_info(filename, strict=args.strict, verbose=args.verbose)

    def range_wrapper(args):
        if args.first < 0:
            raise UsageError("First must be >= 0")
        if args.second <= args.first:
            raise UsageError("Second revision must be > first.")

        try:
            fh = open_or_sys(args.source, sys.stdin, 'rb')
            emit_range(args.source, fh, args.first, args.second,
                       strict=args.strict)
        except IOError as e:
            raise UsageError("Unable to access '%s': %s" % (args.source, e))

    parser = argparse.ArgumentParser("manage svn dumps")

    cmd_parsers = parser.add_subparsers(help="Sub-commands")

    info_parser = add_cmd_parser(cmd_parsers, "info", "Inspect dump(s)", dump_info_wrapper, False)
    info_parser.add_argument("--strict", action="store_true", help="Fail if revisions are not contiguous.")
    info_parser.add_argument("dumpfiles", nargs="+", help="File/path names of dump(s) to inspect")

    join_parser = add_cmd_parser(cmd_parsers, "join", "Join contiguous dumps", join_dumps_wrapper, True)
    join_parser.add_argument("--buf-size", dest="buf_size", type=int, default=16384, help="Buffer size for reads/writes (def: 16384)")
    join_parser.add_argument("--force", action="store_true",
                help="Force append: don't check whether revision numbers are "
                     "contiguous. Use only when you are certain your files "
                     "are sequential: 1-99, 100-199, etc. Does not "
                     "eliminate overlaps. May significantly improve speed.")
    join_parser.add_argument("--rename", "--ren", action="store_true", help="Rename files by adding (or replacing) revision ranges in the filename (e.g svn.100-199.dump)")
    join_parser.add_argument("--rm", action="store_true", help="Remove continuation files after merging them.")
    join_parser.add_argument("--strict", action="store_true", help="Dumps must be exactly contiguous to work.")
    join_parser.add_argument("base", help="Base import file")
    join_parser.add_argument("continuations", nargs="+", help="Files to add to the import")

    sparse_parser = add_cmd_parser(cmd_parsers, "sparse", "Convert a dump into a sparse dump (to stdout)",
                                   lambda args: dump_sparse(args.filename, end_revn=args.end), False)
    sparse_parser.add_argument('--end', type=int, default=None, help="Sparse until this revision")
    sparse_parser.add_argument('filename', help='File to read')

    range_parser = add_cmd_parser(cmd_parsers, "range", "Outputs a new dump using just a given range of revisions.", range_wrapper, False)
    range_parser.add_argument("--strict", action="store_true", help="Require all ranges to be present in source.")
    range_parser.add_argument("first", type=int, help="First revision to include")
    range_parser.add_argument("second", type=int, help="Last revision to include")
    range_parser.add_argument("source", type=str, help="Source file to read from")

    return parser.parse_args(argv)


##############################################################################
# Entry point when used as a script.

def main(argv):
    args = parse_args(sys.argv[1:])

    log_levels = [
        logging.WARN,
        logging.INFO, logging._NOTE, logging._MINOR, logging._SPAM,
        logging.DEBUG
    ]
    args.verbose = min(args.verbose, len(log_levels))
    logging.basicConfig(level=log_levels[args.verbose])

    try:
        args.func(args)
    except UsageError as e:
        sys.stderr.write("ERROR: %s\n" % (str(e)))
        sys.exit(-1)


if __name__ == "__main__":
    main(sys.argv[1:])
