import java.io.IOException;
import java.nio.ByteBuffer;
import java.nio.channels.FileChannel;
import java.nio.channels.FileLock;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.nio.file.StandardOpenOption;

/**
 * Takes session.lock the way Minecraft's DirectoryLock does: open with
 * CREATE+WRITE, write a snowman, force, tryLock (exclusive, whole file).
 *
 *   java HoldLock.java hold  <file>      lock, print "locked", hold until stdin closes
 *   java HoldLock.java cycle <file> <n>  open+lock+release n times, like opening a
 *                                        world repeatedly; exit 1 if a lock is refused
 */
public class HoldLock {
    private static final ByteBuffer DUMMY = ByteBuffer.wrap("☃".getBytes(StandardCharsets.UTF_8));

    static FileChannel open(Path p) throws IOException {
        return FileChannel.open(p, StandardOpenOption.CREATE, StandardOpenOption.WRITE);
    }

    static FileLock lock(FileChannel ch) throws IOException {
        ch.write(DUMMY.duplicate(), 0);
        ch.force(true);
        return ch.tryLock();
    }

    public static void main(String[] args) throws Exception {
        Path file = Path.of(args[1]);
        switch (args[0]) {
            case "hold" -> {
                try (FileChannel ch = open(file)) {
                    FileLock lock = lock(ch);
                    if (lock == null) {
                        System.out.println("refused");
                        System.exit(1);
                    }
                    System.out.println("locked");
                    System.out.flush();
                    while (System.in.read() != -1) { /* hold until the parent closes stdin */ }
                    lock.release();
                }
            }
            case "cycle" -> {
                int n = Integer.parseInt(args[2]);
                for (int i = 0; i < n; i++) {
                    try (FileChannel ch = open(file)) {
                        FileLock lock = lock(ch);
                        if (lock == null) {
                            System.out.println("refused at " + i);
                            System.exit(1);
                        }
                        lock.release();
                    }
                }
                System.out.println("ok");
            }
            default -> throw new IllegalArgumentException(args[0]);
        }
    }
}
