namespace SoulsSaveManager.Tools
{
    internal static class ConsoleUtility
    {
        internal const int BoxWidth = 101;

        // Draws a box around lines, optionally with a title set into the top border.
        public static void WriteBox(IEnumerable<string> lines, string? title = null)
        {
            string top = title == null ? "" : $"─────$> {title} <$";
            Console.WriteLine($" ┌{top}{new string('─', Math.Max(BoxWidth - top.Length, 0))}┐");
            foreach (string line in lines)
                Console.WriteLine($" │{line}{new string(' ', Math.Max(BoxWidth - line.Length, 0))}│");
            Console.WriteLine($" └{new string('─', BoxWidth)}┘");
        }

        public static void RemoveLines(int count)
        {
            for (int i = 0; i < count; i++)
                RemoveLine();
        }

        public static void RemoveLine()
        {
            if (Console.CursorTop == 0)
                return;

            Console.SetCursorPosition(0, Console.CursorTop - 1);
            Console.Write(new string(' ', Console.BufferWidth));
            Console.SetCursorPosition(0, Math.Max(Console.CursorTop - 1, 0));
        }
    }
}
