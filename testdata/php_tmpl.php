<!DOCTYPE html>
<html>
<body>
  <h1>Report</h1>
  <?php
    $title = "Home";
    echo strtoupper($title);
  ?>
  <p>Total: <?= $count ?></p>
  <ul>
    <?php foreach ($items as $item): ?>
      <li><?php echo $item; ?></li>
    <?php endforeach; ?>
  </ul>
</body>
</html>
